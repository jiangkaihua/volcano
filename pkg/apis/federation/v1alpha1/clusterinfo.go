/*
Copyright 2021 The Volcano Authors.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=clusterinfos,scope=Cluster,shortName=ci;ci-v1alpha1

// ClusterInfo defines structure of cluster information.
type ClusterInfo struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the cluster info, including cluster name, api server URL, and resources.
	// +optional
	Spec NodeInfo `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// NodeInfo describes node information of cluster
type NodeInfo struct {
	// TotalNodes is the number of all nodes in cluster
	TotalNodes int32 `json:"totalNodes,omitempty" protobuf:"bytes,1,opt,name=totalNodes"`
	// ReadyNodes is resource info of nodes in Ready status
	ReadyNodes ResourceInfo `json:"readyNodes,omitempty" protobuf:"bytes,2,opt,name=readyNodes"`
	// LabeledNodes is resource info of nodes with specified labels, and in Ready status
	LabeledNodes map[string]ResourceInfo `json:"labeledNodes,omitempty" protobuf:"bytes,3,opt,name=labeledNodes"`
	// UpdateTimeStamp is time stamp when node resource info is updated
	UpdateTimeStamp metav1.Time `json:"updateTimeStamp,omitempty" protobuf:"bytes,4,opt,name=updateTimeStamp"`
}

// ResourceInfo describes detailed resource information of nodes
type ResourceInfo struct {
	// Nodes the number of nodes with specified label in cluster
	Nodes int32 `json:"nodes,omitempty" protobuf:"bytes,1,opt,name=nodes"`
	// Allocatable is the available resource amount of a cluster, which should be the sum of idle & used resources
	Allocatable v1.ResourceList `json:"allocatable,omitempty" protobuf:"bytes,2,opt,name=allocatable"`
	// Used is the used resource amount of a cluster, which are occupied by running pods
	Used v1.ResourceList `json:"used,omitempty" protobuf:"bytes,3,opt,name=used"`
	// Idle is the idle resource amount of a cluster
	Idle v1.ResourceList `json:"idle,omitempty" protobuf:"bytes,4,opt,name=idle"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// ClusterInfoList is a collection of cluster info.
type ClusterInfoList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// items is the list of PodGroup
	Items []ClusterInfo `json:"items" protobuf:"bytes,2,rep,name=items"`
}
