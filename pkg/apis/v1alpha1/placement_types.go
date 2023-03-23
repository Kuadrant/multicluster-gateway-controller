/*
Copyright 2023.
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

// PlacementSpec defines the desired state of Placement
type PlacementSpec struct {
	// Static list of clusters
	Clusters []string `json:"clusters,omitempty"`

	// Copied partially from https://github.com/open-cluster-management-io/api/blob/main/cluster/v1beta1/types_placement.go

	// Predicates represent a slice of predicates to select clusters. The predicates are ORed.
	// This field is optional upstream in ocm, but required here while it is the only way to select clusters
	// +required
	Predicates []ClusterPredicate `json:"predicates,omitempty"`

	// +kubebuilder:validation:Required
	// +required
	TargetRef PlacementTargetReference `json:"targetRef"`
}

// ClusterPredicate represents a predicate to select clusters.
type ClusterPredicate struct {
	// RequiredClusterSelector represents a selector of clusters by label. If specified,
	// 1) Any cluster, which does not match the selector, should not be selected by this ClusterPredicate;
	// 2) If a selected cluster (of this ClusterPredicate) ceases to match the selector (e.g. due to
	//    an update) of any ClusterPredicate, it will be eventually removed from the placement decisions;
	// 3) If a cluster (not selected previously) starts to match the selector, it will either
	//    be selected or at least has a chance to be selected (when NumberOfClusters is specified);
	// +optional
	RequiredClusterSelector ClusterSelector `json:"requiredClusterSelector,omitempty"`
}

type PlacementTargetReference struct {
	// Group is the group of the target resource. e.g. 'gateway.networking.k8s.io'
	// +required
	Group string `json:"group"`

	// Version is the group of the target resource. e.g. 'v1beta1'
	// +required
	Version string `json:"version"`

	// Resource is kind of the target resource. e.g. 'Gateway'
	// +required
	Resource string `json:"resource"`

	// Name is the name of the target resource. Must be in the same namespace as the Placement.
	// +required
	Name string `json:"name"`
}

// ClusterSelector represents the AND of the containing selectors. An empty cluster selector matches all objects.
// A null cluster selector matches no objects.
type ClusterSelector struct {
	// LabelSelector represents a selector of clusters by label
	// This field is optional upstream in ocm, but required here while it is the only way to select clusters
	// +required
	LabelSelector metav1.LabelSelector `json:"labelSelector,omitempty"`
}

// PlacementStatus defines the observed state of Placement
type PlacementStatus struct {
	// NumberOfSelectedClusters represents the number of selected clusters
	// +optional
	NumberOfSelectedClusters int32 `json:"numberOfSelectedClusters"`

	// Conditions contains the different condition status for this Placement.
	// +optional
	Conditions []metav1.Condition `json:"conditions"`

	// Decisions is a slice of decisions according to a placement
	// Upstream in ocm, this is abstracted out to a PlacementDecision resource.
	// +kubebuilder:validation:Required
	// +required
	Decisions []ClusterDecision `json:"decisions"`
}

const (
	// PlacementConditionSatisfied means Placement requirements are satisfied.
	// A placement is not satisfied only if there is empty ClusterDecision in the status.decisions
	// of PlacementDecisions.
	PlacementConditionSatisfied string = "PlacementSatisfied"
	// PlacementConditionMisconfigured means Placement configuration is incorrect.
	PlacementConditionMisconfigured string = "PlacementMisconfigured"
)

// ClusterDecision represents a decision from a placement
// An empty ClusterDecision indicates it is not scheduled yet.
type ClusterDecision struct {
	// ClusterName is the name of the cluster.
	// +kubebuilder:validation:Required
	// +required
	ClusterName string `json:"clusterName"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Placement is the Schema for the placements API
type Placement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlacementSpec   `json:"spec,omitempty"`
	Status PlacementStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PlacementList contains a list of Placement
type PlacementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Placement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Placement{}, &PlacementList{})
}
