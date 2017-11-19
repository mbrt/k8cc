package controller

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/kube"
)

// NewDeployController creates a new controller with the given options and components
func NewDeployController(opts AutoScaleOptions,
	deployer kube.Deployer,
	storage Storage,
	logger log.Logger,
) Controller {
	return &deployController{
		map[string]*deployTagController{},
		opts,
		deployer,
		storage,
		logger,
	}
}

type deployController struct {
	tagControllers map[string]*deployTagController
	autoscaleOpts  AutoScaleOptions
	deployer       kube.Deployer
	storage        Storage
	logger         log.Logger
}

func (c *deployController) DoMaintenance(ctx context.Context, now time.Time) {
	for tag, tc := range c.tagControllers {
		ndeploy, err := tc.doMaintenance(ctx, now)
		if err != nil {
			_ = c.logger.Log("tag", tag, "err", err)
		} else {
			_ = c.logger.Log("tag", tag, "deployments", ndeploy)
		}
	}
}

func (c *deployController) TagController(tag string) TagController {
	return c.getOrMakeTagController(tag)
}

func (c *deployController) getOrMakeTagController(tag string) *deployTagController {
	_, ok := c.tagControllers[tag]
	if !ok {
		new := deployTagController{
			tag,
			c.storage,
			c.autoscaleOpts,
			c.deployer,
		}
		c.tagControllers[tag] = &new
	}
	return c.tagControllers[tag]
}

type deployTagController struct {
	tagName  string
	storage  Storage
	opts     AutoScaleOptions
	deployer kube.Deployer
}

func (c *deployTagController) LeaseUser(ctx context.Context, user string, now time.Time) (Lease, error) {
	t := now.Add(c.opts.LeaseTime)
	c.storage.SetLease(c.tagName, user, t, nil)
	result := Lease{
		Expiration: t,
	}
	return result, nil
}

func (c *deployTagController) doMaintenance(ctx context.Context, now time.Time) (int, error) {
	nactive := c.storage.NumActiveUsers(c.tagName, now)
	replicas := c.computeReplicas(nactive)
	err := c.deployer.ScaleDeploy(ctx, c.tagName, replicas)
	return replicas, err
}

func (c *deployTagController) computeReplicas(numUsers int) int {
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
