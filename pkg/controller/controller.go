//go:generate mockgen -destination mock/controller_mock.go github.com/mbrt/k8cc/pkg/controller Controller,TagController

package controller

import (
	"context"
	"time"
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
	LeaseUser(ctx context.Context, user string, now time.Time) (Lease, error)
}

// Lease contains info about a lease for a specific user and tag
type Lease struct {
	Expiration time.Time
	Hosts      []string
}

// AutoScaleOptions contains scaling options
type AutoScaleOptions struct {
	MinReplicas     int
	MaxReplicas     int
	ReplicasPerUser int
	LeaseTime       time.Duration
}
