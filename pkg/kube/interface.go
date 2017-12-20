//go:generate mockgen -destination mock/kube_mock.go github.com/mbrt/k8cc/pkg/kube Operator

package kube

import (
	"github.com/mbrt/k8cc/pkg/data"
)

const (
	// StatefulSetLabel is a label to identify the stateful sets managed by this controller
	StatefulSetLabel = "k8cc.io/deploy-version"
	// StatefulSetVersion is the version of the stateful sets
	StatefulSetVersion = "v1"
	// BuildTagLabel is the tag that identifies a certain build tag
	BuildTagLabel = "k8cc.io/build-tag"
)

// Operator manages kubernetes objects for distcc tags, by scaling their replicas
type Operator interface {
	Run(threadiness int, stopCh <-chan struct{}) error
	NotifyUpdated(t data.Tag) error
	Hostnames(t data.Tag, ids []data.HostID) ([]string, error)
	ScaleSettings(t data.Tag) (data.ScaleSettings, error)
}
