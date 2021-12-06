/*
Copyright 2021.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FMAppSpec defines the desired state of FMApp
type FMAppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format:=string
	DeploymentName string `json:"deploymentName,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Format:=string
	Image string `json:"image"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	Replicas *int32 `json:"replicas"`
}

// FMAppStatus defines the observed state of FMApp
type FMAppStatus struct { // INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AvailableReplicas   int32  `json:"replicas"`
	VirtualServiceState string `json:"virtualservicestate"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FMApp is the Schema for the fmapps API
type FMApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FMAppSpec   `json:"spec,omitempty"`
	Status FMAppStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FMAppList contains a list of FMApp
type FMAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FMApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FMApp{}, &FMAppList{})
}
