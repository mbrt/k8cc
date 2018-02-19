package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/data"
	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

var (
	// ErrCanceled is used when the request cannot be satisfied on time
	ErrCanceled = errors.New("timeout or canceled")
)

var backoff = wait.Backoff{
	Duration: 5 * time.Millisecond,
	Factor:   2.0,
	Jitter:   1.0,
	Steps:    10,
}

// NewKubeBackend creates a backend that applies the configuration to a Kubernetes cluster
func NewKubeBackend(sharedclient *controller.SharedClient, logger log.Logger) Backend {
	return &kubeBackend{
		k8ccclientset: sharedclient.K8ccClientset,
		logger:        logger,
	}
}

type kubeBackend struct {
	k8ccclientset clientset.Interface
	logger        log.Logger
}

func (b *kubeBackend) LeaseDistcc(ctx context.Context, user data.User, tag data.Tag) (Lease, error) {
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
		return Lease{}, errors.Wrap(err, "failed to get Distcc object")
	}
	distcc := obj.(*k8ccv1alpha1.Distcc)

	_, err = getWithRetry(ctx, rchan, b.logger, func() (interface{}, error) {
		return nil, b.renewDistccLease(ctx, user, distcc)
	})

	host := fmt.Sprintf("%s.%s", distcc.Spec.ServiceName, tag.Namespace)
	res := Lease{
		Expiration: time.Now().Add(distcc.Spec.LeaseDuration.Duration),
		Endpoints:  []string{host},
		Replicas:   int(distcc.Spec.UserReplicas),
	}
	return res, err
}

func (b *kubeBackend) renewDistccLease(ctx context.Context, user data.User, distcc *k8ccv1alpha1.Distcc) error {
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
		return k8ccerr.TransientError(errors.Wrap(err, "get/create claim failed"))
	}

	// Ask for a lease renew if necessary
	if claim.Status.ExpirationTime != nil {
		claim.Status.ExpirationTime = nil
		_, err = b.k8ccclientset.K8ccV1alpha1().DistccClaims(namespace).Update(claim)
	}

	return k8ccerr.TransientError(errors.Wrap(err, "update claim failed"))
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
			_ = logger.Log("context timed out, waiting for last retry...")
			<-rchan
			return false, ErrCanceled
		case rerr := <-rchan:
			if rerr != nil {
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
		TypeMeta: metav1.TypeMeta{
			APIVersion: "k8cc.io/v1alpha1",
			Kind:       "DistccClaim",
		},
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

func makeClaimName(u data.User, resourceName string) string {
	return fmt.Sprintf("%s-%s", u, resourceName)
}
