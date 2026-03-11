/*
Copyright 2026 OpenClaw Platform.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenClawInstanceSpec defines the desired state of OpenClawInstance
type OpenClawInstanceSpec struct {
	// InstanceID is the unique identifier for this instance
	// +kubebuilder:validation:Required
	InstanceID string `json:"instanceId"`

	// UserID is the owner user ID
	// +kubebuilder:validation:Required
	UserID string `json:"userId"`

	// Image configuration
	// +kubebuilder:default="openclaw/openclaw:latest"
	// +optional
	Image string `json:"image,omitempty"`

	// Resources specifies compute resources
	// +optional
	Resources ResourceRequirements `json:"resources,omitempty"`

	// Config is the OpenClaw configuration (openclaw.json)
	// +optional
	Config string `json:"config,omitempty"`

	// Plan is the subscription plan (free/pro/team)
	// +kubebuilder:default="free"
	// +optional
	Plan string `json:"plan,omitempty"`

	// ExpiresAt is the instance expiration time
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`
}

// ResourceRequirements defines CPU and memory resources
type ResourceRequirements struct {
	// CPU request (e.g., "250m", "1")
	// +kubebuilder:default="250m"
	// +optional
	CPU string `json:"cpu,omitempty"`

	// Memory request (e.g., "512Mi", "2Gi")
	// +kubebuilder:default="512Mi"
	// +optional
	Memory string `json:"memory,omitempty"`

	// CPU limit
	// +kubebuilder:default="1000m"
	// +optional
	CPULimit string `json:"cpuLimit,omitempty"`

	// Memory limit
	// +kubebuilder:default="2Gi"
	// +optional
	MemoryLimit string `json:"memoryLimit,omitempty"`
}

// OpenClawInstanceStatus defines the observed state of OpenClawInstance
type OpenClawInstanceStatus struct {
	// Phase is the current lifecycle phase
	// +kubebuilder:validation:Enum=Pending;Creating;Running;Stopped;Error;Deleting
	// +optional
	Phase string `json:"phase,omitempty"`

	// PodName is the name of the managed Pod
	// +optional
	PodName string `json:"podName,omitempty"`

	// HostIP is the node IP where the Pod is running
	// +optional
	HostIP string `json:"hostIP,omitempty"`

	// HostPort is the exposed port (always 18789)
	// +kubebuilder:default=18789
	// +optional
	HostPort int32 `json:"hostPort,omitempty"`

	// GatewayEndpoint is the full access URL
	// +optional
	GatewayEndpoint string `json:"gatewayEndpoint,omitempty"`

	// LastHealthCheck is the timestamp of last health check
	// +optional
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// Conditions represent the latest observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the last reconciled generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="UserID",type=string,JSONPath=`.spec.userId`
// +kubebuilder:printcolumn:name="Plan",type=string,JSONPath=`.spec.plan`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.gatewayEndpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OpenClawInstance is the Schema for the openclawinstances API
type OpenClawInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenClawInstanceSpec   `json:"spec,omitempty"`
	Status OpenClawInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenClawInstanceList contains a list of OpenClawInstance
type OpenClawInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenClawInstance `json:"items"`
}

// Phase constants
const (
	PhasePending    = "Pending"
	PhaseCreating   = "Creating"
	PhaseRunning    = "Running"
	PhaseStopped    = "Stopped"
	PhaseError      = "Error"
	PhaseDeleting   = "Deleting"
)

// Condition types
const (
	ConditionTypeReady       = "Ready"
	ConditionTypeHealthCheck = "HealthCheck"
)

func init() {
	SchemeBuilder.Register(&OpenClawInstance{}, &OpenClawInstanceList{})
}
