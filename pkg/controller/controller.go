//go:generate mockgen -destination mock/controller_mock.go github.com/mbrt/k8cc/pkg/controller Controller,TagController

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
	// TagController returns the controller for the given tag
	TagController(tag string) TagController
}

// TagController manages a single tag deployment
type TagController interface {
	// LeaseUser gives the given user another lease
	LeaseUser(user string, now time.Time) (Lease, error)
}

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	Expiration time.Time
	Hosts      []string
}

// NewDeployController creates a new controller with the given options and components
func NewDeployController(opts AutoScaleOptions,
	leaseTime time.Duration,
	deployer kube.Deployer,
	storage Storage,
	logger log.Logger,
) Controller {
	return &deployController{
		map[string]*deployTagController{},
		leaseTime,
		opts,
		deployer,
		storage,
		logger,
	}
}

type deployController struct {
	tagControllers map[string]*deployTagController
	leaseTime      time.Duration
	autoscaleOpts  AutoScaleOptions
	deployer       kube.Deployer
	storage        Storage
	logger         log.Logger
}

func (c *deployController) DoMaintenance(ctx context.Context, now time.Time) {
	for tag, tc := range c.tagControllers {
		ndeploy, err := tc.DoMaintenance(ctx, now)
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
			c.leaseTime,
			c.storage,
			NewAutoScaler(c.autoscaleOpts, tag, c.deployer),
		}
		c.tagControllers[tag] = &new
	}
	return c.tagControllers[tag]
}

type deployTagController struct {
	tagName   string
	leaseTime time.Duration
	storage   Storage
	scaler    AutoScaler
}

func (c *deployTagController) LeaseUser(user string, now time.Time) (Lease, error) {
	t := now.Add(c.leaseTime)
	c.storage.SetLease(c.tagName, user, t)
	result := Lease{
		Expiration: t,
	}
	return result, nil
}

func (c *deployTagController) DoMaintenance(ctx context.Context, now time.Time) (int, error) {
	nactive := c.storage.NumActiveUsers(c.tagName, now)
	return c.scaler.UpdateUsers(ctx, nactive)
}
