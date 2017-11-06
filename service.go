package k8cc

import (
	"context"
	"net"
	"time"

	"github.com/pkg/errors"
)

var (
	// ErrCanceled is used when the request cannot be satisfied on time
	ErrCanceled = errors.New("timeout or canceled")
)

// Service is an interface that implements all the APIs.
type Service interface {
	Hosts(ctx context.Context, tag string) ([]Host, error)
	LeaseUser(ctx context.Context, user, tag string) (time.Time, error)
}

// NewService creates the API service
func NewService(d Deployer, c Controller) Service {
	return service{d, c}
}

// Host contains information about a build host
type Host struct {
	IP net.IP `json:"ip"`
}

type service struct {
	dep        Deployer
	controller Controller
}

func (s service) Hosts(ctx context.Context, tag string) ([]Host, error) {
	type k8Result struct {
		ips []net.IP
		err error
	}
	ch := make(chan k8Result)
	defer close(ch)

	go func() {
		r, e := s.dep.PodIPs(tag)
		ch <- k8Result{r, e}
	}()

	var ips []net.IP

	select {
	case out := <-ch:
		ips = out.ips
		if out.err != nil {
			return nil, errors.Wrap(out.err, "error retrieving build host IPs")
		}
	case <-ctx.Done():
		return nil, ErrCanceled
	}

	result := make([]Host, len(ips))
	for i, s := range ips {
		result[i] = Host{s}
	}
	return result, nil
}

func (s service) LeaseUser(ctx context.Context, user, tag string) (time.Time, error) {
	expiration := s.controller.LeaseUser(user, tag)
	return expiration, nil
}
