//go:generate mockgen -destination mock/controller_mock.go github.com/mbrt/k8cc/pkg/controller Controller,TagController

package controller

import (
	"context"
	"time"

	"github.com/mbrt/k8cc/pkg/data"
)

// Controller manages the scaling of all the controlled deployments
type Controller interface {
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

// ScaleSettingsProvider provides the scale settings for any given tag
type ScaleSettingsProvider interface {
	// ScaleSettings provides scale settings for a tag
	ScaleSettings(t data.Tag) (data.ScaleSettings, error)
}

// NewStaticScaleSettingsProvider creates a provider that always return the same settings
func NewStaticScaleSettingsProvider(s data.ScaleSettings) ScaleSettingsProvider {
	return staticScaleSettingsProvider(s)
}

type staticScaleSettingsProvider data.ScaleSettings

func (s staticScaleSettingsProvider) ScaleSettings(t data.Tag) (data.ScaleSettings, error) {
	return data.ScaleSettings(s), nil
}
