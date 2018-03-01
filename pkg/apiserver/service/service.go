package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/data"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

var (
	// ErrCanceled is used when the request cannot be satisfied on time
	ErrCanceled = errors.New("timeout or canceled")
	// ErrNotFound is used when the given resource was not found
	ErrNotFound = errors.New("resource not found")
)

var backoff = wait.Backoff{
	Duration: 5 * time.Millisecond,
	Factor:   2.0,
	Jitter:   1.0,
	Steps:    12,
}

// Service is an interface that implements all the APIs.
type Service interface {
	LeaseDistcc(ctx context.Context, u data.User, t data.Tag) (DistccLease, error)
	DeleteDistcc(ctx context.Context, u data.User, t data.Tag) error
	LeaseClient(ctx context.Context, u data.User, t data.Tag) (ClientLease, error)
	DeleteClient(ctx context.Context, u data.User, t data.Tag) error
}

// NewService creates the API service
func NewService(sharedclient *controller.SharedClient, logger log.Logger) Service {
	return service{
		kubeclientset: sharedclient.KubeClientset,
		k8ccclientset: sharedclient.K8ccClientset,
		logger:        logger,
	}
}

// DistccLease contains info about a lease for a specific user and tag.
type DistccLease struct {
	// Expiration is the time at which the lease will end.
	Expiration time.Time `json:"expiration"`
	// Endpoints is the list of hosts that can be used to access the resoure
	// being leased.
	Endpoints []string `json:"endpoints,omitempty"`
	// Replicas is the number of replicas sitting behind the endpoints.
	//
	// This might be useful to determine the number of replicas behind the
	// given endpoints.
	Replicas int `json:"replicas,omitempty"`
}

// ClientLease contains info about a lease for a specific user and tag.
type ClientLease struct {
	// Expiration is the time at which the lease will end.
	Expiration time.Time `json:"expiration"`
	// NodePort is the port allocated for the service.
	//
	// The service will be accessible from any host with an external
	// interface running on the cluster.
	NodePort int `json:"nodePort,omitempty"`
}

type service struct {
	kubeclientset kubernetes.Interface
	k8ccclientset clientset.Interface
	logger        log.Logger
}

func (b service) LeaseDistcc(ctx context.Context, user data.User, tag data.Tag) (DistccLease, error) {
	rchan := make(chan error)
	defer close(rchan)

	obj, err := getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		d, err := b.k8ccclientset.K8ccV1alpha1().Distccs(tag.Namespace).Get(tag.Name, metav1.GetOptions{})
		// No retries in case the distcc object is not there,
		// probably the tag was wrong
		if kubeerr.IsNotFound(err) {
			return nil, err
		}
		return d, k8ccerr.TransientError(err)
	})
	if err != nil {
		return DistccLease{}, errors.Wrap(err, "failed to get Distcc object")
	}
	distcc := obj.(*k8ccv1alpha1.Distcc)

	_, err = getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		return nil, b.renewDistccLease(ctx, user, distcc)
	})

	host := fmt.Sprintf("%s.%s", distcc.Spec.ServiceName, tag.Namespace)
	res := DistccLease{
		Expiration: time.Now().Add(distcc.Spec.LeaseDuration.Duration),
		Endpoints:  []string{host},
		Replicas:   int(distcc.Spec.UserReplicas),
	}
	return res, err
}

func (b service) DeleteDistcc(ctx context.Context, user data.User, tag data.Tag) error {
	rchan := make(chan error)
	defer close(rchan)

	_, err := getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		claimName := makeClaimName(user, tag.Name)
		// Try to delete the resource if present, otherwise 404
		err := b.k8ccclientset.K8ccV1alpha1().DistccClaims(tag.Namespace).Delete(claimName, nil)
		if kubeerr.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, k8ccerr.TransientError(err)
	})

	return err
}

func (b service) LeaseClient(ctx context.Context, user data.User, tag data.Tag) (ClientLease, error) {
	rchan := make(chan error)
	defer close(rchan)

	obj, err := getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		dc, err := b.k8ccclientset.K8ccV1alpha1().DistccClients(tag.Namespace).Get(tag.Name, metav1.GetOptions{})
		// No retries in case the distcc object is not there,
		// probably the tag was wrong
		if kubeerr.IsNotFound(err) {
			return nil, err
		}
		return dc, k8ccerr.TransientError(err)
	})
	if err != nil {
		return ClientLease{}, errors.Wrap(err, "failed to get DistccClient object")
	}
	client := obj.(*k8ccv1alpha1.DistccClient)

	res, err := getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		return b.renewDistccClientLease(ctx, user, client)
	})
	if err != nil {
		return ClientLease{}, err
	}

	return *res.(*ClientLease), err
}

func (b service) DeleteClient(ctx context.Context, user data.User, tag data.Tag) error {
	rchan := make(chan error)
	defer close(rchan)

	_, err := getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		claimName := makeClaimName(user, tag.Name)
		// Try to delete the resource if present, otherwise 404
		err := b.k8ccclientset.K8ccV1alpha1().DistccClientClaims(tag.Namespace).Delete(claimName, nil)
		if kubeerr.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, k8ccerr.TransientError(err)
	})

	return err
}

func (b *service) renewDistccLease(ctx context.Context, user data.User, distcc *k8ccv1alpha1.Distcc) error {
	// TODO use context when supported by client-go
	// see https://github.com/kubernetes/community/pull/1166

	// Try to get an existing claim
	namespace := distcc.Namespace
	claimName := makeClaimName(user, distcc.Name)
	claim, err := b.k8ccclientset.K8ccV1alpha1().DistccClaims(namespace).Get(claimName, metav1.GetOptions{})
	if kubeerr.IsNotFound(err) {
		// It was not present: create it
		new := newDistccClaim(user, distcc)
		claim, err = b.k8ccclientset.K8ccV1alpha1().DistccClaims(namespace).Create(new)
	}
	if err != nil {
		return k8ccerr.TransientError(errors.Wrap(err, "get/create distcc claim failed"))
	}

	// Ask for a lease renew if necessary
	if claim.Status.ExpirationTime != nil {
		claim.Status.ExpirationTime = nil
		_, err = b.k8ccclientset.K8ccV1alpha1().DistccClaims(namespace).Update(claim)
	}

	return k8ccerr.TransientError(errors.Wrap(err, "update claim failed"))
}

func (b *service) renewDistccClientLease(ctx context.Context, user data.User, client *k8ccv1alpha1.DistccClient) (*ClientLease, error) {
	// TODO use context when supported by client-go
	// see https://github.com/kubernetes/community/pull/1166

	// Try to get an existing claim
	namespace := client.Namespace
	claimName := makeClaimName(user, client.Name)
	claim, err := b.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Get(claimName, metav1.GetOptions{})
	if kubeerr.IsNotFound(err) {
		// It was not present: create it

		// Try to get a corresponding secret for the user
		secretName := fmt.Sprintf("%s-ssh-key", user)
		_, err = b.kubeclientset.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
		if err != nil {
			if !kubeerr.IsNotFound(err) {
				return nil, k8ccerr.TransientError(errors.Wrap(err, "get secret failed"))
			}
			// No secrets for the user: fine, we just don't use secrets
			secretName = ""
		}

		// Create the claim at this point
		new := newDistccClientClaim(user, client, secretName)
		claim, err = b.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Create(new)
	}
	if err != nil {
		return nil, k8ccerr.TransientError(errors.Wrap(err, "get/create client claim failed"))
	}

	// Ask for a lease renew if necessary
	if claim.Status.ExpirationTime != nil {
		claim.Status.ExpirationTime = nil
		_, err = b.k8ccclientset.K8ccV1alpha1().DistccClientClaims(namespace).Update(claim)
	}
	if err != nil {
		return nil, k8ccerr.TransientError(errors.Wrap(err, "update claim failed"))
	}

	// Wait for the status to be updated
	var service *corev1.Service
	if service, err = b.getServiceForClaim(claim); err != nil {
		return nil, err
	}

	res := ClientLease{
		Expiration: time.Now().Add(client.Spec.LeaseDuration.Duration),
		NodePort:   int(service.Spec.Ports[0].NodePort),
	}

	return &res, nil
}

func (b *service) getServiceForClaim(claim *k8ccv1alpha1.DistccClientClaim) (*corev1.Service, error) {
	// Wait for the status to be updated
	if claim.Status.Service == nil {
		return nil, k8ccerr.TransientError(errors.New("service for claim not present yet"))
	}
	if claim.Status.Deployment == nil {
		return nil, k8ccerr.TransientError(errors.New("deploy for claim not present yet"))
	}

	service, err := b.kubeclientset.CoreV1().Services(claim.Namespace).Get(claim.Status.Service.Name, metav1.GetOptions{})
	if err != nil {
		return nil, k8ccerr.TransientError(errors.New("service for claim not present yet"))
	}
	if len(service.Spec.Ports) == 0 || service.Spec.Ports[0].NodePort == 0 {
		return nil, k8ccerr.TransientError(errors.New("service for claim has no node port yet"))
	}

	return service, nil
}

type getterFunc func() (obj interface{}, err error)

func getWithRetry(ctx context.Context, rchan chan error, logger log.Logger, getter getterFunc) (interface{}, error) {
	var object interface{}
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		go func() {
			var err error
			object, err = getter()
			rchan <- err
		}()
		select {
		case <-ctx.Done():
			/* #nosec */
			_ = logger.Log("context timed out, waiting for last retry...")
			<-rchan
			return false, ErrCanceled
		case rerr := <-rchan:
			if rerr != nil {
				/* #nosec */
				_ = logger.Log("err", rerr)
				// If the error is transient we want to retry, but if not we return immediately
				if !k8ccerr.IsTransient(rerr) {
					return false, rerr
				}
			}
			return rerr == nil, nil
		}
	})

	return object, err
}

func newDistccClaim(user data.User, distcc *k8ccv1alpha1.Distcc) *k8ccv1alpha1.DistccClaim {
	return &k8ccv1alpha1.DistccClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      makeClaimName(user, distcc.Name),
			Namespace: distcc.Namespace,
			Labels:    distcc.Labels,
		},
		Spec: k8ccv1alpha1.DistccClaimSpec{
			DistccName: distcc.Name,
			UserName:   string(user),
		},
	}
}

func newDistccClientClaim(
	user data.User,
	client *k8ccv1alpha1.DistccClient,
	secretName string,
) *k8ccv1alpha1.DistccClientClaim {

	toPtr := func(i int) *int32 {
		r := int32(i)
		return &r
	}

	res := &k8ccv1alpha1.DistccClientClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      makeClaimName(user, client.Name),
			Namespace: client.Namespace,
			Labels:    client.Labels,
		},
		Spec: k8ccv1alpha1.DistccClientClaimSpec{
			DistccClientName: client.Name,
			UserName:         string(user),
		},
	}

	if len(secretName) > 0 {
		res.Spec.Secrets = []k8ccv1alpha1.Secret{
			{
				Name: "ssh",
				VolumeSource: corev1.SecretVolumeSource{
					SecretName:  secretName,
					DefaultMode: toPtr(0755),
				},
			},
		}
	}

	return res
}

func makeClaimName(u data.User, resourceName string) string {
	return fmt.Sprintf("%s-%s", u, resourceName)
}
