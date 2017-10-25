package k8cc

import (
	"net"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Deployer handles changing and querying kubernetes deployments
type Deployer interface {
	// PodIPs returns the IPs of the Pods running with a certain tag
	PodIPs(tag string) ([]net.IP, error)
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
