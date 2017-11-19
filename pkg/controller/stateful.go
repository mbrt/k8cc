package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/mbrt/k8cc/pkg/kube"
)

var (
	kubeServicePrefix = "k8cc-build"
)

// NewStatefulController creates a controller that uses StatefulSets to manage the build hosts
func NewStatefulController(opts AutoScaleOptions, storage Storage, deployer kube.Deployer) Controller {
	return statefulController{
		opts,
		storage,
		deployer,
	}
}

type statefulController struct {
	opts     AutoScaleOptions
	storage  Storage
	deployer kube.Deployer
}

func (c statefulController) DoMaintenance(ctx context.Context, now time.Time) {
}

func (c statefulController) TagController(tag string) TagController {
	return statefulTagController{tag, c.opts, c.storage, c.deployer}
}

type statefulTagController struct {
	tag      string
	opts     AutoScaleOptions
	storage  Storage
	deployer kube.Deployer
}

func (c statefulTagController) LeaseUser(ctx context.Context, user string, now time.Time) (Lease, error) {
	// find the free host with the lowest id
	usage := c.storage.Usage(c.tag, now)
	assigned := len(usage) // default to the last (non-assigned) one
	for id, n := range usage {
		if n == 0 {
			assigned = id
			break
		}
	}
	hosts := make([]BuildHostID, c.opts.ReplicasPerUser)
	for i := range hosts {
		// we have to wrap around max replicas
		hosts[i] = BuildHostID(assigned % c.opts.MaxReplicas)
		assigned++
	}

	t := now.Add(c.opts.LeaseTime)
	c.storage.SetLease(c.tag, user, t, hosts)

	hostnames := make([]string, len(hosts))
	for i, id := range hosts {
		hostnames[i] = fmt.Sprintf("%s-%s%d", kubeServicePrefix, c.tag, id)
	}

	nactive := c.storage.NumActiveUsers(c.tag, now)
	replicas := c.computeReplicas(nactive)
	err := c.deployer.ScaleSet(ctx, c.tag, replicas)

	result := Lease{
		Expiration: t,
		Hosts:      hostnames,
	}
	return result, err
}

func (c *statefulTagController) computeReplicas(numUsers int) int {
	ideal := numUsers * c.opts.ReplicasPerUser
	switch {
	case ideal < c.opts.MinReplicas:
		return c.opts.MinReplicas
	case ideal > c.opts.MaxReplicas:
		return c.opts.MaxReplicas
	default:
		return ideal
	}
}
