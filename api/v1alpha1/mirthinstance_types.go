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
)

// MirthInstanceSpec defines the desired state of MirthInstance.
type MirthInstanceSpec struct {
	// connection configures how to reach the Mirth Connect REST API.
	// +required
	Connection ConnectionSpec `json:"connection"`

	// monitoring configures how often to poll Mirth and whether to expose Prometheus metrics.
	// +required
	Monitoring MonitoringSpec `json:"monitoring"`

	// remediation configures automatic self-healing of unhealthy channels.
	// +optional
	Remediation RemediationSpec `json:"remediation,omitempty"`
}

// ConnectionSpec defines the Mirth Connect REST API connection details.
type ConnectionSpec struct {
	// host is the hostname or IP of the Mirth Connect server.
	// +required
	Host string `json:"host"`

	// port is the Mirth Connect REST API port (typically 8443).
	// +kubebuilder:default=8443
	// +optional
	Port int `json:"port,omitempty"`

	// tls configures TLS settings for the connection.
	// +optional
	TLS TLSSpec `json:"tls,omitempty"`

	// authSecretRef references a Kubernetes Secret containing "username" and "password" keys.
	// +required
	AuthSecretRef SecretReference `json:"authSecretRef"`
}

// TLSSpec configures TLS for the Mirth connection.
type TLSSpec struct {
	// insecureSkipVerify disables TLS certificate verification.
	// Use only for self-signed certificates in non-production environments.
	// +optional
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// SecretReference points to a Kubernetes Secret.
type SecretReference struct {
	// name is the name of the Secret in the same namespace as the MirthInstance.
	// +required
	Name string `json:"name"`
}

// MonitoringSpec configures the monitoring interval and metrics.
type MonitoringSpec struct {
	// intervalSeconds is how often (in seconds) the operator polls Mirth for status.
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=5
	// +optional
	IntervalSeconds int `json:"intervalSeconds,omitempty"`

	// metrics configures Prometheus metrics exposure.
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`
}

// MetricsSpec configures Prometheus metrics.
type MetricsSpec struct {
	// enabled controls whether Prometheus metrics are collected for this instance.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`
}

// RemediationSpec configures automatic channel remediation.
type RemediationSpec struct {
	// enabled controls whether automatic remediation is active.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// restartStoppedChannels restarts channels that are in STOPPED state.
	// +optional
	RestartStoppedChannels bool `json:"restartStoppedChannels,omitempty"`

	// restartErroredChannels restarts channels that are in ERROR state.
	// +optional
	RestartErroredChannels bool `json:"restartErroredChannels,omitempty"`

	// maxRestartAttempts is the maximum number of restart attempts per channel
	// before giving up. 0 means unlimited.
	// +kubebuilder:default=3
	// +optional
	MaxRestartAttempts int `json:"maxRestartAttempts,omitempty"`

	// cooldownSeconds is the minimum time between restart attempts for the same channel.
	// +kubebuilder:default=300
	// +optional
	CooldownSeconds int `json:"cooldownSeconds,omitempty"`

	// excludeChannels is a list of channel IDs or names to exclude from remediation.
	// +optional
	ExcludeChannels []string `json:"excludeChannels,omitempty"`
}

// MirthInstanceStatus defines the observed state of MirthInstance.
type MirthInstanceStatus struct {
	// conditions represent the current state of the MirthInstance.
	// Condition types: Connected, AllChannelsHealthy, RemediationActive
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// server contains Mirth server information.
	// +optional
	Server ServerStatus `json:"server,omitempty"`

	// channels is a summary of channel health.
	// +optional
	Channels ChannelSummary `json:"channels,omitempty"`

	// channelDetails lists the status of each individual channel.
	// +optional
	ChannelDetails []ChannelStatus `json:"channelDetails,omitempty"`

	// remediationHistory records recent remediation actions.
	// +optional
	RemediationHistory []RemediationEvent `json:"remediationHistory,omitempty"`

	// lastChecked is the last time the operator successfully polled Mirth.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
}

// ServerStatus contains information about the Mirth server.
type ServerStatus struct {
	// version is the Mirth Connect version string.
	// +optional
	Version string `json:"version,omitempty"`

	// status is the Mirth server status (e.g. "RUNNING").
	// +optional
	Status string `json:"status,omitempty"`

	// jvmHeapUsedBytes is the current JVM heap usage.
	// +optional
	JVMHeapUsedBytes int64 `json:"jvmHeapUsedBytes,omitempty"`
}

// ChannelSummary provides an overview of channel health.
type ChannelSummary struct {
	// total is the total number of channels.
	// +optional
	Total int `json:"total,omitempty"`

	// started is the number of channels in STARTED state.
	// +optional
	Started int `json:"started,omitempty"`

	// stopped is the number of channels in STOPPED state.
	// +optional
	Stopped int `json:"stopped,omitempty"`

	// errored is the number of channels in ERROR state.
	// +optional
	Errored int `json:"errored,omitempty"`

	// paused is the number of channels in PAUSED state.
	// +optional
	Paused int `json:"paused,omitempty"`
}

// ChannelStatus represents the status of a single Mirth channel.
type ChannelStatus struct {
	// id is the Mirth channel UUID.
	ID string `json:"id"`

	// name is the channel display name.
	Name string `json:"name"`

	// state is the channel state (STARTED, STOPPED, PAUSED, ERROR).
	State string `json:"state"`

	// received is the total number of messages received.
	// +optional
	Received int64 `json:"received,omitempty"`

	// sent is the total number of messages sent.
	// +optional
	Sent int64 `json:"sent,omitempty"`

	// errored is the total number of errored messages.
	// +optional
	Errored int64 `json:"errored,omitempty"`

	// queued is the current queue depth.
	// +optional
	Queued int64 `json:"queued,omitempty"`

	// filtered is the total number of filtered messages.
	// +optional
	Filtered int64 `json:"filtered,omitempty"`
}

// RemediationEvent records a single remediation action.
type RemediationEvent struct {
	// channelID is the Mirth channel UUID.
	ChannelID string `json:"channelId"`

	// channelName is the channel display name.
	ChannelName string `json:"channelName"`

	// action is what was done (e.g. "start", "restart").
	Action string `json:"action"`

	// result is the outcome ("success" or "failure").
	Result string `json:"result"`

	// message provides additional details.
	// +optional
	Message string `json:"message,omitempty"`

	// timestamp is when the remediation occurred.
	Timestamp metav1.Time `json:"timestamp"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Connected",type=string,JSONPath=`.status.conditions[?(@.type=="Connected")].status`
// +kubebuilder:printcolumn:name="Channels",type=integer,JSONPath=`.status.channels.total`
// +kubebuilder:printcolumn:name="Healthy",type=integer,JSONPath=`.status.channels.started`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// MirthInstance is the Schema for the mirthinstances API.
type MirthInstance struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of MirthInstance.
	// +required
	Spec MirthInstanceSpec `json:"spec"`

	// status defines the observed state of MirthInstance.
	// +optional
	Status MirthInstanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// MirthInstanceList contains a list of MirthInstance.
type MirthInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []MirthInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MirthInstance{}, &MirthInstanceList{})
}
