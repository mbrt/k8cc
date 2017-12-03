package kube

import (
	"github.com/mbrt/k8cc/pkg/data"
)

// Operator manages kubernetes objects for distcc tags, by scaling their replicas
type Operator interface {
	Run(threadiness int, stopCh <-chan struct{}) error
	NotifyUpdated(t data.Tag) error
	Hostnames(t data.Tag, ids []data.HostID) ([]string, error)
}
