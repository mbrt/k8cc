package algo

import (
	"time"

	"github.com/mbrt/k8cc/pkg/controller/distccold"
	"github.com/mbrt/k8cc/pkg/data"
	"github.com/mbrt/k8cc/pkg/state"
)

// Adapter adapts Controller to the interface used by Operator and vice-versa.
// This is used to solve the circular dependency between the two objects
type Adapter struct {
	Controller Controller
	Operator   distccold.Operator
	State      state.TagsStater
}

// Replicas is the adapter method for DesiredState interface
func (a Adapter) Replicas(t data.Tag) int32 {
	r := a.Controller.TagController(t).DesiredReplicas(time.Now())
	return int32(r)
}

// Leases is the adapter method for the DesiredState interface
func (a Adapter) Leases(t data.Tag) []data.Lease {
	return a.State.TagState(t).Leases(time.Now())
}

// Hostnames is the adapter method for the Hostnamer interface
func (a Adapter) Hostnames(t data.Tag, ids []data.HostID) ([]string, error) {
	return a.Operator.Hostnames(t, ids)
}

// ScaleSettings is the adapter method for the ScaleSettingsProvider interface
func (a Adapter) ScaleSettings(t data.Tag) (data.ScaleSettings, error) {
	return a.Operator.ScaleSettings(t)
}
