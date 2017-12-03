//go:generate mockgen -destination mock/deployer_mock.go github.com/mbrt/k8cc/pkg/kube Deployer

// see https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
// and https://github.com/kubernetes/sample-controller/blob/master/controller.go

package kube

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/mbrt/k8cc/pkg/data"
)

const (
	// ServicePrefix is the prefix for a build hostname
	ServicePrefix = "k8cc-build"
	// StatefulSetLabel is a label to identify the stateful sets managed by this controller
	StatefulSetLabel = "k8cc.io/deploy-version"
	// StatefulSetVersion is the version of the stateful sets
	StatefulSetVersion = "v1"
	// BuildTagLabel is the tag that identifies a certain build tag
	BuildTagLabel = "k8cc.io/build-tag"
	// MaxRetries is the maximal number of retries before to give up an update
	MaxRetries = 10
)

// Deployer handles changing and querying kubernetes deployments
type Deployer interface {
	// DeploymentName returns the name of the deployment that serves the given tag.
	DeploymentName(tag data.Tag) string
	// ScaleSet scales a stateful set to the given replica count
	ScaleSet(ctx context.Context, tag data.Tag, replicas int) error
	// DeploymentsState returns the state of all controlled deployments
	DeploymentsState(ctx context.Context) ([]DeploymentState, error)
}

// DeploymentState contains the state of the deployment for a tag
type DeploymentState struct {
	Tag      data.Tag
	Replicas int
}

// NewKubeDeployer creates a Deployer able to talk to an in cluster Kubernetes deployer
func NewKubeDeployer(ns string) (Deployer, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "error in cluster config")
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create client")
	}
	return inClusterDeployer{clientset, ns}, nil
}

// BuildHostname returns the hostname for the given tag and host ID
func BuildHostname(tag data.Tag, id data.HostID) string {
	return fmt.Sprintf("%s-%s%d", ServicePrefix, tag, id)
}

type inClusterDeployer struct {
	clientset *kubernetes.Clientset
	namespace string
}

func (d inClusterDeployer) ScaleSet(ctx context.Context, tag data.Tag, replicas int) error {
	setsClient := d.clientset.AppsV1beta2().StatefulSets(d.namespace)
	setName := d.DeploymentName(tag)
	set, err := setsClient.Get(setName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot get stateful set for tag %s", tag))
	}
	if set.Status.Replicas == int32(replicas) {
		// no need to update
		return nil
	}

	//    You have two options to Update() this Deployment:
	//
	//    1. Modify the "set" variable and call: Update(set).
	//       This works like the "kubectl replace" command and it overwrites/loses changes
	//       made by other clients between you Create() and Update() the object.
	//    2. Modify the "result" returned by Create()/Get() and retry Update(result) until
	//       you no longer get a conflict error. This way, you can preserve changes made
	//       by other clients between Create() and Update(). This is implemented below:

	limiter := rate.NewLimiter(3.0, 2)
	retries := 0
	for {
		set.Spec.Replicas = int32Ptr(replicas)

		if _, err := setsClient.Update(set); k8errors.IsConflict(err) {
			// Deployment is modified in the meanwhile, query the latest version
			// and modify the retrieved object.
			fmt.Println("encountered conflict, retrying")
			set, err = setsClient.Get(setName, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("get now failed for tag %s", tag))
			}
		} else if err != nil {
			return errors.Wrap(err, fmt.Sprintf("stateful set update failed for tag %s", tag))
		} else {
			break
		}

		// Sleep here with an exponential backoff to avoid
		// exhausting the apiserver, and add a limit/timeout on the retries to
		// avoid getting stuck in this loop indefintiely.
		retries++
		if retries > MaxRetries {
			return errors.New("cannot update stateful set; max retries reached")
		}
		if err := limiter.Wait(ctx); err != nil {
			return errors.Wrap(err, "context expired while updating stateful set")
		}
	}

	return nil
}

func (d inClusterDeployer) DeploymentName(tag data.Tag) string {
	return fmt.Sprintf("k8cc-build-%s", tag)
}

func (d inClusterDeployer) DeploymentsState(ctx context.Context) ([]DeploymentState, error) {
	setsClient := d.clientset.AppsV1beta2().StatefulSets(d.namespace)
	ls := labels.Set{StatefulSetLabel: StatefulSetVersion}
	lsOptions := metav1.ListOptions{LabelSelector: ls.AsSelector().String()}
	sets, err := setsClient.List(lsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error listing stateful sets")
	}

	result := []DeploymentState{}
	for _, ss := range sets.Items {
		tag, ok := ss.Spec.Template.Labels[BuildTagLabel]
		if !ok {
			return nil, fmt.Errorf("missing build label from stateful set %s", ss.Name)
		}
		replicas := ss.Status.Replicas
		result = append(result, DeploymentState{data.Tag(tag), int(replicas)})
	}

	return result, nil
}

func int32Ptr(u int) *int32 {
	i := int32(u)
	return &i
}
