package controller

import (
	"time"

	"github.com/mbrt/k8cc/pkg/data"
	"github.com/mbrt/k8cc/pkg/kube"
)

// Adapter adapts Controller to the interface used by Operator and vice-versa.
// This is used to solve the circular dependency between the two objects
type Adapter struct {
	Controller Controller
	Operator   kube.Operator
}

// DesiredReplicas is the adapter method for DesiredReplicasCache interface
func (a Adapter) DesiredReplicas(t data.Tag) int32 {
	r := a.Controller.TagController(t).DesiredReplicas(time.Now())
	return int32(r)
}

// Hostnames is the adapter method for the Hostnamer interface
func (a Adapter) Hostnames(t data.Tag, ids []data.HostID) ([]string, error) {
	return a.Operator.Hostnames(t, ids)
}
