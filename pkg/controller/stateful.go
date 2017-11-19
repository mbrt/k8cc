package controller

import (
	"context"
	"time"

	"github.com/mbrt/k8cc/pkg/kube"
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
	t := now.Add(c.opts.LeaseTime)
	c.storage.SetLease(c.tag, user, t)
	nactive := c.storage.NumActiveUsers(c.tag, now)
	replicas := c.computeReplicas(nactive)
	err := c.deployer.ScaleSet(ctx, c.tag, replicas)
	if err != nil {
		return Lease{Expiration: t}, err
	}
	result := Lease{
		Expiration: t,
	}
	return result, nil
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
