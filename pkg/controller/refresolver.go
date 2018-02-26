package controller

import (
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	kubeerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	k8ccerr "github.com/mbrt/k8cc/pkg/errors"
)

// Lister is a capable of listing runtime objects of a certain type.
type Lister interface {
	// ListByName returns the object by name
	ListByName(namespace, name string) (runtime.Object, error)
	// ListBySelector returns all the objects satisfying the given selector
	ListBySelector(namespace string, selector labels.Selector) ([]runtime.Object, error)
}

// NewDeploymentLister returns a lister for deployments
func NewDeploymentLister(ls appslisters.DeploymentLister) Lister {
	return deploymentLister{lister: ls}
}

// NewServiceLister returns a lister for deployments
func NewServiceLister(ls corelisters.ServiceLister) Lister {
	return serviceLister{lister: ls}
}

// GetObjectFromReferenceOrSelector returns an object referenced by the given reference, or by the selector.
//
// Among the possible errors, TransientError is returned if it's not possible to reply at this point,
// or AmbiguousReferenceError if more than one object satisfies the selector.
func GetObjectFromReferenceOrSelector(lister Lister, namespace string, ref *corev1.LocalObjectReference, selector labels.Selector) (runtime.Object, error) {
	obj, err := GetObjectFromReference(lister, namespace, ref)
	if err != nil || obj != nil {
		return obj, err
	}
	return GetObjectFromLabels(lister, namespace, selector)
}

// GetObjectFromReference returns the object pointed by the reference, if it exists.
//
// Among the possible errors, TransientError is returned if it's not possible to reply at this point.
func GetObjectFromReference(lister Lister, namespace string, ref *corev1.LocalObjectReference) (runtime.Object, error) {
	if ref == nil {
		return nil, nil
	}
	obj, err := lister.ListByName(namespace, ref.Name)
	if err != nil {
		if kubeerr.IsNotFound(err) {
			return nil, nil
		}
		return nil, k8ccerr.TransientError(err)
	}
	return obj, nil
}

// GetObjectFromLabels returns the only object satisfying the required selector.
//
// Among the possible errors, TransientError is returned if it's not possible to reply at this point,
// or AmbiguousReferenceError if more than one object satisfies the selector.
func GetObjectFromLabels(lister Lister, namespace string, selector labels.Selector) (runtime.Object, error) {
	objs, err := lister.ListBySelector(namespace, selector)
	if err != nil {
		if kubeerr.IsNotFound(err) {
			return nil, nil
		}
		return nil, k8ccerr.TransientError(err)
	}
	if len(objs) == 0 {
		return nil, nil
	}
	if len(objs) > 1 {
		return nil, AmbiguousReferenceError(fmt.Errorf("multiple objects (%d) satisfy the label selector, expected only one", len(objs)))
	}
	return objs[0], nil
}

// IsAmbiguousReference returns true if an error is ambiguous reference
func IsAmbiguousReference(err error) bool {
	ae, ok := errors.Cause(err).(ambiguous)
	return ok && ae.AmbiguousReference()
}

// AmbiguousReferenceError wraps the given error and makes it into an ambiguous reference one
func AmbiguousReferenceError(err error) error {
	return ambiguousReferenceError{err}
}

type ambiguous interface {
	AmbiguousReference() bool
}

type ambiguousReferenceError struct {
	error
}

func (e ambiguousReferenceError) Error() string            { return e.error.Error() }
func (e ambiguousReferenceError) AmbiguousReference() bool { return true }

type deploymentLister struct {
	lister appslisters.DeploymentLister
}

func (d deploymentLister) ListByName(namespace, name string) (runtime.Object, error) {
	return d.lister.Deployments(namespace).Get(name)
}

func (d deploymentLister) ListBySelector(namespace string, selector labels.Selector) ([]runtime.Object, error) {
	ds, err := d.lister.Deployments(namespace).List(selector)
	res := make([]runtime.Object, len(ds))
	for i, d := range ds {
		res[i] = d
	}
	return res, err
}

type serviceLister struct {
	lister corelisters.ServiceLister
}

func (s serviceLister) ListByName(namespace, name string) (runtime.Object, error) {
	return s.lister.Services(namespace).Get(name)
}

func (s serviceLister) ListBySelector(namespace string, selector labels.Selector) ([]runtime.Object, error) {
	ds, err := s.lister.Services(namespace).List(selector)
	res := make([]runtime.Object, len(ds))
	for i, d := range ds {
		res[i] = d
	}
	return res, err
}
