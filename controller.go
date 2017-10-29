package k8cc

import (
	"time"
)

// UserAccessController manages the state of the users' leases for a deployment
type UserAccessController struct {
	leaseTime  time.Duration
	clock      Clock
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
