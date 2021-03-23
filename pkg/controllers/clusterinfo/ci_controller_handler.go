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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog"
)

const (
	// AddResource defines event action should be add
	AddResource = "add"
	// DeleteResource defines event action should be delete
	DeleteResource = "delete"
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
	for rName, rQuant := range r {
		if _, exist := l[rName]; !exist {
			l[rName] = resource.Quantity{}
		}
		quantity := l[rName]
		quantity.Add(rQuant)
		l[rName] = quantity
	}

	return l
}

// SubResourceList returns v1.ResourceList l subtracts r, l must be larger than r in all dimensions
func SubResourceList(l, r v1.ResourceList) v1.ResourceList {
	for rName, rQuant := range r {
		if _, exist := l[rName]; !exist {
			klog.Errorf("Invalid resource type <%s> for <%v>.", rName, l)
			return nil
		}
		quantity := l[rName]
		if quantity.Cmp(rQuant) == -1 {
			klog.Errorf("Resource <%v> is less than <%v> in type %s.", l, r, rName)
			return nil
		}
		quantity.Sub(rQuant)
		l[rName] = quantity
	}

	return l
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
	if l == nil || r == nil {
		if l == nil {
			return r
		}
		return l
	}

	for rName, rQuant := range r {
		if _, exist := l[rName]; !exist {
			l[rName] = rQuant
		}
		quantity := l[rName]
		if quantity.Cmp(rQuant) == -1 {
			rQuant.DeepCopyInto(&quantity)
			l[rName] = quantity
		}
	}

	return l
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

// createPodEvent creates a new event from pod, and add it into podEventQueue
func (ci cicontroller) createPodEvent(obj interface{}, eventType string) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		klog.Errorf("Failed to convert obj: %v to v1.Pod", obj)
		return
	}

	var event = clusterInfoPodEvent{
		eventType: eventType,
		name:      pod.Name,
		namespace: pod.Namespace,
	}

	ci.podEventQueue.Add(event)
}

// addPod adds a new pod info into clusterInfo
func (ci *cicontroller) addPod(obj interface{}) {
	ci.createPodEvent(obj, AddResource)
}

// updatePod updates an existing pod info in clusterInfo
func (ci *cicontroller) updatePod(oldObj, newObj interface{}) {
	ci.createPodEvent(oldObj, DeleteResource)
	ci.createPodEvent(newObj, AddResource)
}

// deletePod deletes pod info in clusterInfo
func (ci *cicontroller) deletePod(obj interface{}) {
	ci.createPodEvent(obj, DeleteResource)
}

// createNodeEvent creates a new event from node, and add it into nodeEventQueue
func (ci cicontroller) createNodeEvent(obj interface{}, eventType string) {
	node, ok := obj.(*v1.Node)
	if !ok {
		klog.Errorf("Failed to convert obj: %v to v1.Node", obj)
		return
	}

	var event = clusterInfoNodeEvent{
		eventType: eventType,
		name:      node.Name,
		nodeReady: false,
	}
	if node.Status.Phase == v1.NodeRunning {
		event.nodeReady = true
	}

	ci.nodeEventQueue.Add(event)
}

// addNode adds new node info into clusterInfo
func (ci *cicontroller) addNode(obj interface{}) {
	ci.createNodeEvent(obj, AddResource)
}

// updateNode updates existing node info in clusterInfo
func (ci *cicontroller) updateNode(oldObj, newObj interface{}) {
	ci.createNodeEvent(oldObj, DeleteResource)
	ci.createNodeEvent(newObj, AddResource)
}

// deleteNode deletes node info in clusterInfo
func (ci *cicontroller) deleteNode(obj interface{}) {
	ci.createNodeEvent(obj, DeleteResource)
}
