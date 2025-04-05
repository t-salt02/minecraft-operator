/*
Copyright 2025.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MinecraftServerSpec defines the desired state of MinecraftServer.
type MinecraftServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MinecraftServer. Edit minecraftserver_types.go to remove/update
	Seed string `json:"seed,omitempty"`
	// +kubebuilder:validation:Enum=peaceful;easy;normal;hard
	// +kubebuilder:default=normal
	// +kubebuilder:validation:Required
	Difficulty string `json:"difficulty,omitempty"`
	// +kubebuilder:default=latest
	Version    string `json:"version,omitempty"`
	ServerName string `json:"serverName"`
	// +kubebuilder:default=false
	Hardcore bool `json:"hardcore,omitempty"`
	// +kubebuilder:validation:Pattern=`^\d+(Mi|Gi)$`
	// +kubebuilder:default="5Gi"
	Storage string `json:"storage,omitempty"`
}

// MinecraftServerStatus defines the observed state of MinecraftServer.
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=".status.ready",description="Pod readiness"
// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=".status.ip",description="Server IP address"
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=".spec.version",description="Minecraft version"
type MinecraftServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Ready bool   `json:"ready,omitempty"`
	IP    string `json:"ip,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MinecraftServer is the Schema for the minecraftservers API.
type MinecraftServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MinecraftServerSpec   `json:"spec,omitempty"`
	Status MinecraftServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MinecraftServerList contains a list of MinecraftServer.
type MinecraftServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MinecraftServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MinecraftServer{}, &MinecraftServerList{})
}
