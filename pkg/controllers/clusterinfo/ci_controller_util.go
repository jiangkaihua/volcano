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
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// CIName is name of custom resource cluster info, which should be one and only one in whole cluster
	CIName = "cluster-resources-info"
	// CIPeriod is time of round for ci-controller to update cluster info
	CIPeriod = time.Second * 3
	// virtualNode is name of virtual node created by virtual kubelet
	virtualNode = "virtual-kubelet"
)

// CPUPhases is phases to filter nodes with idle CPU number
var CPUPhases = []string{"20", "40", "60"}

// MEMPhases is phases to filter nodes with idle memories
var MEMPhases = []string{"10Gi", "40Gi", "80Gi"}

// AddResourceList adds v1.ResourceList and returns the sum
func AddResourceList(l, r v1.ResourceList) v1.ResourceList {
	if l == nil {
		return r
	}
	if r == nil {
		return l
	}
	var sum = l.DeepCopy()
	for resourceName, resourceQuantity := range r {
		if _, exist := sum[resourceName]; !exist {
			sum[resourceName] = resource.Quantity{}
		}
		quantity := sum[resourceName]
		quantity.Add(resourceQuantity)
		sum[resourceName] = quantity
	}

	return sum
}

// SubResourceList returns v1.ResourceList l subtracts r, l must be larger than r in all dimensions
func SubResourceList(l, r v1.ResourceList) (v1.ResourceList, error) {
	if r == nil {
		return l, nil
	}
	var diff = l.DeepCopy()
	for resourceName, resourceQuantity := range r {
		if value, exist := diff[resourceName]; !exist {
			if !value.IsZero() {
				return nil, fmt.Errorf("resource <%s> do not exist in minuend <%v>", resourceName, l)
			}
		}
		if resourceQuantity.Cmp(diff[resourceName]) == 1 {
			return nil, fmt.Errorf("minuend <%v> is less than subtrahend <%v> in resource <%s>", l, r, resourceName)
		}
		quantity := diff[resourceName]
		quantity.Sub(resourceQuantity)
		diff[resourceName] = quantity
	}

	return diff, nil
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
	for resourceName, resourceQuantity := range r {
		if _, exist := maxResources[resourceName]; !exist {
			maxResources[resourceName] = resource.Quantity{}
		}
		quantity := maxResources[resourceName]
		if quantity.Cmp(resourceQuantity) == -1 {
			maxResources[resourceName] = resourceQuantity.DeepCopy()
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

// CalculateResourcesSum calculates sum of resources on all nodes in map
func CalculateResourcesSum(nodesResources map[string]v1.ResourceList) v1.ResourceList {
	var sum = v1.ResourceList{}
	for _, nodeResources := range nodesResources {
		sum = AddResourceList(sum, nodeResources)
	}
	return sum
}
