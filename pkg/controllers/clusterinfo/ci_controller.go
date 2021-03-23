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
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"volcano.sh/volcano/pkg/apis/federation/v1alpha1"
	vcclientset "volcano.sh/volcano/pkg/client/clientset/versioned"
	"volcano.sh/volcano/pkg/controllers/framework"
)

const CIName = "cluster-resources-info"

func init() {
	framework.RegisterController(&cicontroller{})
}

// cicontroller defines struct of cluster info controller
type cicontroller struct {
	kubeClient kubernetes.Interface
	vcClient   vcclientset.Interface

	// A store of pods
	podInformer coreinformers.PodInformer
	podLister   corelisters.PodLister
	podSynced   func() bool

	// A store of nodes
	nodeInformer coreinformers.NodeInformer
	nodeLister   corelisters.NodeLister
	nodeSynced   func() bool

	// podEventQueue records events from pods that would update clusterInfo
	podEventQueue workqueue.RateLimitingInterface
	// nodeEventQueue records events from nodes that would update clusterInfo
	nodeEventQueue workqueue.RateLimitingInterface

	// a map of sorted slices to save all kinds of resources order, used to update maxResources
	// map[resourceName][]nodeName
}

func (ci *cicontroller) Name() string {
	return "ci-controller"
}

// Initialize creates new ClusterInfo controller
func (ci *cicontroller) Initialize(opt *framework.ControllerOption) error {
	ci.kubeClient = opt.KubeClient
	ci.vcClient = opt.VolcanoClient

	var obj = &v1alpha1.ClusterInfo{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: CIName,
		},
		Spec: v1alpha1.ClusterInfoSpec{},
	}

	if _, err := ci.vcClient.FederationV1alpha1().ClusterInfos().Create(context.TODO(), obj, metav1.CreateOptions{}); err != nil {
		if errors.IsAlreadyExists(err) {
			klog.Infof("Cluster info already exists.")
		} else {
			klog.Errorf("Failed to create cluster info, error: %s.", err.Error())
		}
	}

	ci.podInformer = opt.SharedInformerFactory.Core().V1().Pods()
	ci.podLister = ci.podInformer.Lister()
	ci.podSynced = ci.podInformer.Informer().HasSynced
	ci.podInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch pod := obj.(type) {
				case *v1.Pod:
					if pod.Status.Phase == v1.PodRunning {
						return true
					}
					return false
				default:
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    ci.addPod,
				UpdateFunc: ci.updatePod,
				DeleteFunc: ci.deletePod,
			},
		},
	)

	ci.nodeInformer = opt.SharedInformerFactory.Core().V1().Nodes()
	ci.nodeLister = ci.nodeInformer.Lister()
	ci.nodeSynced = ci.nodeInformer.Informer().HasSynced
	ci.nodeInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ci.addNode,
			UpdateFunc: ci.updateNode,
			DeleteFunc: ci.deleteNode,
		},
	)

	ci.podEventQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	ci.nodeEventQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return nil
}

// Run starts ClusterInfo controller
func (ci *cicontroller) Run(stopCh <-chan struct{}) {
	go ci.podInformer.Informer().Run(stopCh)
	go ci.nodeInformer.Informer().Run(stopCh)

	cache.WaitForCacheSync(stopCh, ci.podSynced, ci.nodeSynced)

	go wait.Until(ci.worker, 0, stopCh)

	klog.Infof("ClusterInfoController is running ...... ")
}

func (ci *cicontroller) worker() {
	clusterInfo, err := ci.vcClient.FederationV1alpha1().ClusterInfos().Get(context.TODO(), CIName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			var obj = &v1alpha1.ClusterInfo{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: CIName,
				},
				Spec: v1alpha1.ClusterInfoSpec{},
			}

			if _, err := ci.vcClient.FederationV1alpha1().ClusterInfos().
				Create(context.TODO(), obj, metav1.CreateOptions{}); err != nil {
				klog.Errorf("Failed to create cluster info, error: %s.", err.Error())
				return
			}
		}
		klog.Errorf("Failed to get clusterInfo, error: %v.", err)
		return
	}

	var resourcesInfo = clusterInfo.Spec.Resources.DeepCopy()
	klog.V(3).Infof("Get resourcesInfo: %v.", resourcesInfo)

	for ci.processNextEvent(*resourcesInfo, ci.nodeEventQueue) {
	}
	klog.V(3).Infof("Update resourcesInfo after node events: %v.", resourcesInfo)

	for ci.processNextEvent(*resourcesInfo, ci.podEventQueue) {
	}
	klog.V(3).Infof("Update resourcesInfo after pod events: %v.", resourcesInfo)

	resourcesInfo.Idle = SubResourceList(resourcesInfo.Allocatable, resourcesInfo.Used)
	klog.V(3).Infof("Update resourcesInfo.Idle: %v.", resourcesInfo.Idle)

	// resourcesInfo.AveragePerNode = resourcesInfo.Allocatable / resourcesInfo.ReadyNodes

	clusterInfo.Spec.Resources = *resourcesInfo
	klog.V(3).Infof("Update clusterInfo: %v.", clusterInfo)

	if _, err := ci.vcClient.FederationV1alpha1().ClusterInfos().
		Update(context.TODO(), clusterInfo, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update cluster info, error: %s", err.Error())
	}
}

func (ci *cicontroller) processNextEvent(resources v1alpha1.ResourcesInfo, queue workqueue.RateLimitingInterface) bool {
	obj, shutdown := queue.Get()
	if shutdown {
		klog.Errorf("Failed to pop item from cluster info controller queue.")
		return false
	}
	klog.V(3).Infof("Dealing with event: %v.", obj)

	switch event := obj.(type) {
	case clusterInfoNodeEvent:
		ci.processNodeEvent(resources, event)
	case clusterInfoPodEvent:
		ci.processPodEvent(resources, event)
	default:
		klog.Errorf("Invalid event in cluster info controller queue.")
	}

	queue.Forget(obj)
	return true
}

func (ci *cicontroller) processPodEvent(resources v1alpha1.ResourcesInfo, event clusterInfoPodEvent) {
	defer ci.podEventQueue.Done(event)

	pod, err := ci.podLister.Pods(event.namespace).Get(event.name)
	if err != nil {
		klog.Errorf("Failed to get pod by <%v> from cache, error: %s", event, err.Error())
		return
	}

	// handle event popped from queue, modify cluster info spec
	switch event.eventType {
	case AddResource:
		// add pod resources into used
		resources.Used = AddResourceList(resources.Used, GetPodResourceRequest(pod))
	case DeleteResource:
		// delete pod resources from used
		resources.Used = SubResourceList(resources.Used, GetPodResourceRequest(pod))
	default:
		klog.Errorf("Invalid item in cluster info controller queue.")
	}
}

func (ci *cicontroller) processNodeEvent(resources v1alpha1.ResourcesInfo, event clusterInfoNodeEvent) {
	defer ci.nodeEventQueue.Done(event)

	node, err := ci.nodeLister.Get(event.name)
	if err != nil {
		klog.Errorf("Failed to get node by <%v> from cache, error: %s", event, err.Error())
		return
	}

	// handle event popped from queue, modify cluster info spec
	switch event.eventType {
	case AddResource:
		resources.TotalNodes++
		if event.nodeReady {
			// add node resources into allocatable
			resources.Allocatable = AddResourceList(resources.Allocatable, node.Status.Allocatable)
			resources.ReadyNodes++
			// update resources.MaxResources
			//resources.MaxResources = SetMaxResources(resources.MaxResources, node.Status.Allocatable)
		}
	case DeleteResource:
		resources.TotalNodes--
		if event.nodeReady {
			// delete node resources from allocatable
			resources.Used = SubResourceList(resources.Allocatable, node.Status.Allocatable)
			resources.ReadyNodes--
			// update resources.MaxResources
		}
	default:
		klog.Errorf("Invalid item in cluster info controller queue.")
	}
}
