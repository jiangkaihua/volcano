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

package clusterinfo

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
)

const (
	// CIName is name of custom resource cluster info, which should be one and only one in whole cluster
	CIName = "cluster-resources-info"
	// CIPeriod is time of round for ci-controller to update cluster info
	CIPeriod = time.Second * 3
	// AddResource defines event action should be add
	AddResource = "add"
	// DeleteResource defines event action should be delete
	DeleteResource = "delete"
	// virtualNode is name of virtual node created by virtual kubelet
	virtualNode = "virtual-kubelet"
)

type clusterInfoPodEvent struct {
	eventType string
	name      string
	namespace string
}

type clusterInfoNodeEvent struct {
	eventType string
	name      string
	nodeReady bool
}

// AddResourceList adds v1.ResourceList and returns the sum
func AddResourceList(l, r v1.ResourceList) v1.ResourceList {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}
	var sum = l.DeepCopy()
	for rName, rQuant := range r {
		if _, exist := sum[rName]; !exist {
			sum[rName] = resource.Quantity{}
		}
		quantity := sum[rName]
		quantity.Add(rQuant)
		sum[rName] = quantity
	}

	return sum
}

// SubResourceList returns v1.ResourceList l subtracts r, l must be larger than r in all dimensions
func SubResourceList(l, r v1.ResourceList) v1.ResourceList {
	if r == nil {
		return l
	}
	var diff = l.DeepCopy()
	for rName, rQuant := range r {
		if _, exist := diff[rName]; !exist {
			klog.Errorf("Invalid resource type <%s> for <%v>.", rName, l)
			return nil
		}
		quantity := diff[rName]
		if quantity.Cmp(rQuant) == -1 {
			klog.Errorf("Resource <%v> is less than <%v> in type %s.", l, r, rName)
			return nil
		}
		quantity.Sub(rQuant)
		diff[rName] = quantity
	}

	return diff
}

// GetPodResourceWithoutInitContainers returns Pod's resource request, it does not contain
// init containers' resource request.
func GetPodResourceWithoutInitContainers(pod *v1.Pod) v1.ResourceList {
	var containersResources = v1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		containersResources = AddResourceList(containersResources, container.Resources.Requests)
	}

	return containersResources
}

// SetMaxResources compares with ResourceList and takes max value for each Resource.
func SetMaxResources(l, r v1.ResourceList) v1.ResourceList {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}

	var maxResources = l.DeepCopy()
	for rName, rQuant := range r {
		if _, exist := maxResources[rName]; !exist {
			maxResources[rName] = rQuant
		}
		quantity := maxResources[rName]
		if quantity.Cmp(rQuant) == -1 {
			rQuant.DeepCopyInto(&quantity)
			maxResources[rName] = quantity
		}
	}

	return maxResources
}

// GetPodResourceRequest returns all the resource required for that pod
func GetPodResourceRequest(pod *v1.Pod) v1.ResourceList {
	podResourceRequest := GetPodResourceWithoutInitContainers(pod)

	// take max_resource(sum_pod, any_init_container)
	for _, initContainer := range pod.Spec.InitContainers {
		podResourceRequest = SetMaxResources(podResourceRequest, initContainer.Resources.Requests)
	}

	return podResourceRequest
}
