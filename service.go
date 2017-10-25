package k8cc

import (
	"context"
	"net"

	"github.com/pkg/errors"
)

// Service is an interface that implements all the APIs.
type Service interface {
	Hosts(ctx context.Context, tag string) ([]Host, error)
}

// NewService creates the API service
func NewService(d Deployer) Service {
	return service{d}
}

// Host contains information about a build host
type Host struct {
	IP net.IP `json:"ip"`
}

type service struct {
	dep Deployer
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
		return nil, errors.New("timeout or canceled")
	}

	result := make([]Host, len(ips))
	for i, s := range ips {
		result[i] = Host{s}
	}
	return result, nil
}
