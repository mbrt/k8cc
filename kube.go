//go:generate mockgen -destination mock/kube_mock.go github.com/mbrt/k8cc Deployer

// see https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
// and https://github.com/kubernetes/sample-controller/blob/master/controller.go

package k8cc

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"golang.org/x/time/rate"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	maxRetries = 10
)

// Deployer handles changing and querying kubernetes deployments
type Deployer interface {
	// PodIPs returns the IPs of the Pods running with a certain tag
	PodIPs(tag string) ([]net.IP, error)
	// ScaleDeploy scales the deployment to the given replica count
	ScaleDeploy(ctx context.Context, tag string, replicas int) error
	// DeploymentName returns the name of the deployment that serves the given tag.
	DeploymentName(tag string) string

	// ScaleSet scales a stateful set to the given replica count
	ScaleSet(ctx context.Context, tag string, replicas int) error
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

type inClusterDeployer struct {
	clientset *kubernetes.Clientset
	namespace string
}

func (d inClusterDeployer) PodIPs(tag string) ([]net.IP, error) {
	// example of handling deployments from the controller:
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/deployment/deployment_controller.go
	podsClient := d.clientset.CoreV1().Pods(d.namespace)
	ls := labels.Set{"k8cc.io/deploy-tag": tag}
	lsOptions := metav1.ListOptions{LabelSelector: ls.AsSelector().String()}
	pods, err := podsClient.List(lsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error listing pods")
	}

	result := []net.IP{}
	for _, pod := range pods.Items {
		if ip := net.ParseIP(pod.Status.PodIP); ip != nil {
			result = append(result, ip)
		}
	}

	return result, nil
}

func (d inClusterDeployer) ScaleDeploy(ctx context.Context, tag string, replicas int) error {
	deploymentsClient := d.clientset.AppsV1beta2().Deployments(d.namespace)
	deployName := d.DeploymentName(tag)

	deploy, err := deploymentsClient.Get(deployName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot get deployment for tag %s", tag))
	}
	if deploy.Status.Replicas == int32(replicas) {
		// no need to update
		return nil
	}

	//    You have two options to Update() this Deployment:
	//
	//    1. Modify the "deployment" variable and call: Update(deployment).
	//       This works like the "kubectl replace" command and it overwrites/loses changes
	//       made by other clients between you Create() and Update() the object.
	//    2. Modify the "result" returned by Create()/Get() and retry Update(result) until
	//       you no longer get a conflict error. This way, you can preserve changes made
	//       by other clients between Create() and Update(). This is implemented below:

	limiter := rate.NewLimiter(3.0, 2)
	retries := 0
	for {
		deploy.Spec.Replicas = int32Ptr(replicas)

		if _, err := deploymentsClient.Update(deploy); k8errors.IsConflict(err) {
			// Deployment is modified in the meanwhile, query the latest version
			// and modify the retrieved object.
			fmt.Println("encountered conflict, retrying")
			deploy, err = deploymentsClient.Get(deployName, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("get now failed for tag %s", tag))
			}
		} else if err != nil {
			return errors.Wrap(err, fmt.Sprintf("deploy update failed for tag %s", tag))
		} else {
			break
		}

		// Sleep here with an exponential backoff to avoid
		// exhausting the apiserver, and add a limit/timeout on the retries to
		// avoid getting stuck in this loop indefintiely.
		retries++
		if retries > maxRetries {
			return errors.New("cannot update deployment; max retries reached")
		}
		if err := limiter.Wait(ctx); err != nil {
			return errors.Wrap(err, "context expired while updating deployment")
		}
	}

	return nil
}

func (d inClusterDeployer) ScaleSet(ctx context.Context, tag string, replicas int) error {
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
		if retries > maxRetries {
			return errors.New("cannot update stateful set; max retries reached")
		}
		if err := limiter.Wait(ctx); err != nil {
			return errors.Wrap(err, "context expired while updating stateful set")
		}
	}

	return nil
}

func (d inClusterDeployer) DeploymentName(tag string) string {
	return fmt.Sprintf("deploy-%s", tag)
}

func int32Ptr(u int) *int32 {
	i := int32(u)
	return &i
}
