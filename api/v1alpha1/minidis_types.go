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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// MinidisSpec defines the desired state of a minidis instance.
type MinidisSpec struct {
	// version is the minidis image tag to run.
	// +kubebuilder:default="1.0.0"
	// +optional
	Version string `json:"version,omitempty"`
	// storage configures the persistent volume that holds the AOF.
	// It cannot be changed after creation.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="storage cannot be changed after creation"
	// +kubebuilder:default={}
	// +optional
	Storage MinidisStorage `json:"storage,omitzero"`
	// resources sets the CPU and memory requests and limits of the minidis container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// auth selects the Secret key that holds the password clients must send with AUTH.
	// Authentication is disabled when it is not set.
	// +optional
	Auth *corev1.SecretKeySelector `json:"auth,omitempty"`
	// fsync is the AOF fsync policy: always, everysec or no.
	// +kubebuilder:validation:Enum=always;everysec;no
	// +kubebuilder:default=everysec
	// +optional
	Fsync string `json:"fsync,omitempty"`
}

// MinidisStorage configures the persistent volume of a minidis instance.
type MinidisStorage struct {
	// size is the requested size of the volume.
	// +kubebuilder:default="2Gi"
	// +optional
	Size resource.Quantity `json:"size,omitzero"`
	// storageClassName is the StorageClass of the volume. The cluster default is used when it is not set.
	// +optional
	StorageClassName *string `json:"storageClassName,omitempty"`
}

// MinidisStatus defines the observed state of a minidis instance.
type MinidisStatus struct {
	// conditions represent the current state of the instance.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// observedGeneration is the metadata.generation of the Minidis that was last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// readyReplicas is the number of minidis Pods that are ready, 0 or 1.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
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
