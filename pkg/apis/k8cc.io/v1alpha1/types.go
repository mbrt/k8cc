/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=distcc

// Distcc is a specification for a Distcc resource
type Distcc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DistccSpec   `json:"spec"`
	Status DistccStatus `json:"status"`
}

// DistccSpec is the spec for a Distcc resource
type DistccSpec struct {
	DeploymentName  string                `json:"deploymentName"`
	ServiceName     string                `json:"serviceName"`
	Selector        *metav1.LabelSelector `json:"selector,omitempty"`
	MinReplicas     *int32                `json:"minReplicas,omitempty"`
	MaxReplicas     int32                 `json:"maxReplicas"`
	UserReplicas    int32                 `json:"userReplicas"`
	LeaseDuration   metav1.Duration       `json:"leaseDuration"`
	DownscaleWindow *metav1.Duration      `json:"downscaleWindow"`
	Template        v1.PodTemplateSpec    `json:"template"`
}

// DistccStatus is the status for a Distcc resource
type DistccStatus struct {
	LastUpdateTime *metav1.Time  `json:"lastUpdateTime,omitempty"`
	LastScaleTime  *metav1.Time  `json:"lastScaleTime,omitempty"`
	Leases         []DistccLease `json:"leases,omitempty"`
}

// DistccLease contains info about a user lease
type DistccLease struct {
	UserName       string      `json:"userName"`
	ExpirationTime metav1.Time `json:"expirationTime"`
	// AssignedHosts represents the ID of the hosts assigned to the user
	AssignedHosts []int32 `json:"assignedHosts"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +resource:path=distccs

// DistccList is a list of Distcc resources
type DistccList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Distcc `json:"items"`
}
