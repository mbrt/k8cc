package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k8ccv1alpha1 "github.com/mbrt/k8cc/pkg/apis/k8cc.io/v1alpha1"
	clientset "github.com/mbrt/k8cc/pkg/client/clientset/versioned"
	"github.com/mbrt/k8cc/pkg/controller"
	"github.com/mbrt/k8cc/pkg/data"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/wait"
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

func (b *kubeBackend) LeaseDistcc(ctx context.Context, user data.User, tag data.Tag) (Host, error) {
	distcc, err := b.k8ccclientset.K8ccV1alpha1().Distccs(tag.Namespace).Get(tag.Name, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to get Distcc object")
	}
	vchan := make(chan error)
	err = wait.ExponentialBackoff(backoff, func() (bool, error) {
		go func() {
			vchan <- b.renewDistccLease(ctx, user, distcc)
		}()
		select {
		case <-ctx.Done():
			_ = b.logger.Log("contenxt timed out")
			return false, ErrCanceled
		case err := <-vchan:
			if err != nil {
				_ = b.logger.Log("err", err)
			}
			return err == nil, nil
		}
	})
	host := fmt.Sprintf("%s.%s", distcc.Spec.ServiceName, tag.Namespace)
	return Host(host), err
}

func (b *kubeBackend) renewDistccLease(ctx context.Context, user data.User, distcc *k8ccv1alpha1.Distcc) error {
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
		return errors.Wrap(err, "get/create claim failed")
	}

	// Ask for a lease renew if necessary
	if claim.Status.ExpirationTime != nil {
		claim.Status.ExpirationTime = nil
		_, err = b.k8ccclientset.K8ccV1alpha1().DistccClaims(namespace).Update(claim)
	}

	return errors.Wrap(err, "update claim failed")
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

func makeClaimName(u data.User, resourceName string) string {
	return fmt.Sprintf("%s-%s", u, resourceName)
}
