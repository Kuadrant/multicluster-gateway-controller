/*
Copyright 2023 The MultiCluster Traffic Controller Authors.

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

package v1alpha2

// +kubebuilder:validation:Enum=None;Secret;ManagedZone
type ProviderKind string

type ProviderRef struct {
	//+required
	Name string `json:"name"`
	//+required
	Kind ProviderKind `json:"kind"`
}

const (
	ProviderKindNone        = "None"
	ProviderKindSecret      = "Secret"
	ProviderKindManagedZone = "ManagedZone"
)

// +kubebuilder:object:generate=false
type ProviderAccessor interface {
	GetNamespace() string
	GetProviderRef() ProviderRef
}
