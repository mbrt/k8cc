package controller

import (
	"context"
	"time"
)

// TagController manages the scaling of a single tag
type TagController struct {
	uac    UserAccessController
	scaler AutoScaler
}

// LeaseUser resets the timer for the user and gives them another lease
func (c *TagController) LeaseUser(user string, now time.Time) time.Time {
	return c.uac.LeaseUser(user, now)
}

// DoMaintenance does the deployment scaling based on the number of active users for the tag
func (c *TagController) DoMaintenance(ctx context.Context, now time.Time) (int, error) {
	nactive := c.uac.ActiveUsers(now)
	return c.scaler.UpdateUsers(ctx, nactive)
}

// UserAccessController manages the state of the users' leases for a deployment
type UserAccessController struct {
	leaseTime time.Duration
	// maps a user to a lease time
	userLeases map[string]time.Time
}

// NewUserAccessController creates a UserAccessController with the given options
func NewUserAccessController(lease time.Duration) UserAccessController {
	return UserAccessController{
		lease,
		map[string]time.Time{},
	}
}

// LeaseUser renews the lease of the given user
func (c *UserAccessController) LeaseUser(name string, now time.Time) time.Time {
	t := now.Add(c.leaseTime)
	c.userLeases[name] = t
	return t
}

// ActiveUsers returns the number of active users
func (c *UserAccessController) ActiveUsers(now time.Time) int {
	for user, expTime := range c.userLeases {
		if expTime.Before(now) {
			delete(c.userLeases, user)
		}
	}
	return len(c.userLeases)
}
