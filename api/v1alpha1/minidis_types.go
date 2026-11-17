/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// MinidisSpec defines the desired state of a minidis instance.
type MinidisSpec struct {
}

// MinidisStatus defines the observed state of a minidis instance.
type MinidisStatus struct {
	// conditions represent the current state of the instance.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Minidis is a single minidis server with its storage and Services.
type Minidis struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Minidis
	// +required
	Spec MinidisSpec `json:"spec"`

	// status defines the observed state of Minidis
	// +optional
	Status MinidisStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MinidisList contains a list of Minidis
type MinidisList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Minidis `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Minidis{}, &MinidisList{})
		return nil
	})
}
