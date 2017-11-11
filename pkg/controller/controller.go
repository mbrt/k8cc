//go:generate mockgen -destination mock/controller_mock.go github.com/mbrt/k8cc/pkg/controller Controller

package controller

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"

	"github.com/mbrt/k8cc/pkg/kube"
)

// Controller manages the scaling of all the controlled deployments
type Controller interface {
	// DoMaintenance takes care of scaling the deployments based on the active users
	DoMaintenance(ctx context.Context, now time.Time)
	// LeaseUser gives the given user another lease for the given tag
	LeaseUser(user, tag string, now time.Time) time.Time
}

type controller struct {
	tagControllers map[string]*TagController
	leaseTime      time.Duration
	autoscaleOpts  AutoScaleOptions
	deployer       kube.Deployer
	logger         log.Logger
}

// NewController creates a new controller with the given options and components
func NewController(opts AutoScaleOptions, leaseTime time.Duration, d kube.Deployer, l log.Logger) Controller {
	return &controller{
		map[string]*TagController{},
		leaseTime,
		opts,
		d,
		l,
	}
}

func (c *controller) DoMaintenance(ctx context.Context, now time.Time) {
	for tag, tc := range c.tagControllers {
		ndeploy, err := tc.DoMaintenance(ctx, now)
		if err != nil {
			_ = c.logger.Log("tag", tag, "err", err)
		} else {
			_ = c.logger.Log("tag", tag, "deployments", ndeploy)
		}
	}
}

func (c *controller) LeaseUser(user, tag string, now time.Time) time.Time {
	tc := c.getOrMakeTagController(tag)
	return tc.LeaseUser(user, now)
}

func (c *controller) getOrMakeTagController(tag string) *TagController {
	_, ok := c.tagControllers[tag]
	if !ok {
		new := TagController{
			NewUserAccessController(c.leaseTime),
			NewAutoScaler(c.autoscaleOpts, tag, c.deployer),
		}
		c.tagControllers[tag] = &new
	}
	return c.tagControllers[tag]
}
