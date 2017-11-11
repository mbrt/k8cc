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
	storage        Storage
	logger         log.Logger
}

// NewController creates a new controller with the given options and components
func NewController(opts AutoScaleOptions,
	leaseTime time.Duration,
	deployer kube.Deployer,
	storage Storage,
	logger log.Logger,
) Controller {
	return &controller{
		map[string]*TagController{},
		leaseTime,
		opts,
		deployer,
		storage,
		logger,
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
			tag,
			c.leaseTime,
			c.storage,
			NewAutoScaler(c.autoscaleOpts, tag, c.deployer),
		}
		c.tagControllers[tag] = &new
	}
	return c.tagControllers[tag]
}

// TagController manages the scaling of a single tag
type TagController struct {
	tagName   string
	leaseTime time.Duration
	storage   Storage
	scaler    AutoScaler
}

// LeaseUser resets the timer for the user and gives them another lease
func (c *TagController) LeaseUser(user string, now time.Time) time.Time {
	t := now.Add(c.leaseTime)
	c.storage.SetLease(c.tagName, user, t)
	return t
}

// DoMaintenance does the deployment scaling based on the number of active users for the tag
func (c *TagController) DoMaintenance(ctx context.Context, now time.Time) (int, error) {
	nactive := c.storage.NumActiveUsers(c.tagName, now)
	return c.scaler.UpdateUsers(ctx, nactive)
}
