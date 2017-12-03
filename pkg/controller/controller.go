//go:generate mockgen -destination mock/controller_mock.go github.com/mbrt/k8cc/pkg/controller Controller,TagController

package controller

import (
	"context"
	"time"

	"github.com/mbrt/k8cc/pkg/data"
)

// Controller manages the scaling of all the controlled deployments
type Controller interface {
	// DoMaintenance takes care of scaling the deployments based on the active users
	DoMaintenance(ctx context.Context, now time.Time)
	// TagController returns the controller for the given tag
	TagController(t data.Tag) TagController
}

// TagController manages a single tag deployment
type TagController interface {
	// LeaseUser gives the given user another lease
	LeaseUser(ctx context.Context, u data.User, now time.Time) (Lease, error)
	// DesiredReplicas returns the desired number of replicas for this tag
	DesiredReplicas(now time.Time) int
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
