package k8cc

import (
	"context"
	"time"

	"github.com/go-kit/kit/log"
)

// Controller manages the scaling of all the controlled deployments
type Controller struct {
	tagControllers map[string]*TagController
	clock          Clock
	leaseTime      time.Duration
	autoscaleOpts  AutoScaleOptions
	deployer       Deployer
}

// DoMaintenance takes care of scaling the deployments based on the active users
func (c *Controller) DoMaintenance(ctx context.Context, logger log.Logger) {
	for tag, tc := range c.tagControllers {
		ndeploy, err := tc.DoMaintenance(ctx)
		if err != nil {
			_ = logger.Log("tag", tag, "err", err)
		} else {
			_ = logger.Log("tag", tag, "deployments", ndeploy)
		}
	}

}

// LeaseUser gives the given user another lease for the given tag
func (c *Controller) LeaseUser(user, tag string) {
	tc := c.getOrMakeTagController(tag)
	tc.LeaseUser(user)
}

func (c *Controller) getOrMakeTagController(tag string) *TagController {
	_, ok := c.tagControllers[tag]
	if !ok {
		new := TagController{
			NewUserAccessController(c.leaseTime, c.clock),
			NewAutoScaler(c.autoscaleOpts, tag, c.deployer),
		}
		c.tagControllers[tag] = &new
	}
	return c.tagControllers[tag]
}

// TagController manages the scaling of a single tag
type TagController struct {
	uac    UserAccessController
	scaler AutoScaler
}

// LeaseUser resets the timer for the user and gives them another lease
func (c *TagController) LeaseUser(user string) {
	c.uac.LeaseUser(user)
}

// DoMaintenance does the deployment scaling based on the number of active users for the tag
func (c *TagController) DoMaintenance(ctx context.Context) (uint, error) {
	nactive := c.uac.ActiveUsers()
	return c.scaler.UpdateUsers(ctx, nactive)
}

// UserAccessController manages the state of the users' leases for a deployment
type UserAccessController struct {
	leaseTime time.Duration
	clock     Clock
	// maps a user to a lease time
	userLeases map[string]time.Time
}

// Clock provides access to the system time
type Clock interface {
	// Now returns the current time
	Now() time.Time
}

// NewUserAccessController creates a UserAccessController with the given options
func NewUserAccessController(lease time.Duration, clock Clock) UserAccessController {
	return UserAccessController{
		lease,
		clock,
		map[string]time.Time{},
	}
}

// DefaultUserAccessController creates a UserAccessController with the given options
func DefaultUserAccessController(lease time.Duration) UserAccessController {
	return UserAccessController{
		lease,
		systemClock{},
		map[string]time.Time{},
	}
}

// LeaseUser renews the lease of the given user
func (c *UserAccessController) LeaseUser(name string) time.Time {
	t := c.clock.Now().Add(c.leaseTime)
	c.userLeases[name] = t
	return t
}

// ActiveUsers returns the number of active users
func (c *UserAccessController) ActiveUsers() uint {
	now := c.clock.Now()
	for user, expTime := range c.userLeases {
		if expTime.Before(now) {
			delete(c.userLeases, user)
		}
	}
	return uint(len(c.userLeases))
}

type systemClock struct{}

func (s systemClock) Now() time.Time {
	return time.Now()
}
