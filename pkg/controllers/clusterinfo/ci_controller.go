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
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
		if !errors.IsAlreadyExists(err) {
			klog.Errorf("Failed to create cluster info, error: %s.", err.Error())
		}
	}

	ci.podInformer = opt.SharedInformerFactory.Core().V1().Pods()
	ci.podLister = ci.podInformer.Lister()
	ci.podSynced = ci.podInformer.Informer().HasSynced

	ci.nodeInformer = opt.SharedInformerFactory.Core().V1().Nodes()
	ci.nodeLister = ci.nodeInformer.Lister()
	ci.nodeSynced = ci.nodeInformer.Informer().HasSynced

	ci.podEventQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	ci.nodeEventQueue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return nil
}

// Run starts ClusterInfo controller
func (ci *cicontroller) Run(stopCh <-chan struct{}) {
	go ci.podInformer.Informer().Run(stopCh)
	go ci.nodeInformer.Informer().Run(stopCh)

	cache.WaitForCacheSync(stopCh, ci.podSynced, ci.nodeSynced)

	go wait.Until(ci.worker, CIPeriod, stopCh)

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
		} else {
			klog.Errorf("Failed to get clusterInfo, error: %v.", err)
			return
		}
	}

	var newClusterInfo = clusterInfo.DeepCopy()
	var resourcesInfo = &v1alpha1.ResourcesInfo{
		Allocatable:      nil,
		Used:             nil,
		Idle:             nil,
		TotalNodes:       0,
		ReadyNodes:       0,
		AveragePerNode:   nil,
		MaxResources:     nil,
		NodesWithIdleCPU: make(map[string]int32, len(CPUPhases)),
		NodesWithIdleMem: make(map[string]int32, len(MEMPhases)),
	}

	nodes, err := ci.nodeLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list all nodes, error: %s.", err.Error())
		return
	}

	resourcesInfo.TotalNodes = int32(len(nodes))
	// allocatableResourcesOnNodes is total allocatable resources on ready nodes
	var allocatableResourcesOnNodes = make(map[string]v1.ResourceList, len(nodes))
	for _, node := range nodes {
		if node.Name == virtualNode {
			continue
		}
		if node.Status.Conditions[len(node.Status.Conditions)-1].Type == v1.NodeReady {
			resourcesInfo.ReadyNodes++
			var allocatable = node.Status.Allocatable
			resourcesInfo.MaxResources = SetMaxResources(resourcesInfo.MaxResources, allocatable)
			allocatableResourcesOnNodes[node.Name] = allocatable
		}
	}

	pods, err := ci.podLister.List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list all pods, error: %s.", err.Error())
		return
	}

	// usedResourcesOnNodes is used resources occupied by pods on nodes
	// should be less than allocatable resources on the node
	var usedResourcesOnNodes = make(map[string]v1.ResourceList, len(allocatableResourcesOnNodes))
	for _, pod := range pods {
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			continue
		}
		if _, exist := allocatableResourcesOnNodes[pod.Spec.NodeName]; !exist {
			continue
		}
		usedResourcesOnNodes[pod.Spec.NodeName] =
			AddResourceList(usedResourcesOnNodes[pod.Spec.NodeName], GetPodResourceRequest(pod))
	}

	var idleResourcesOnNodes = make(map[string]v1.ResourceList, len(allocatableResourcesOnNodes))
	for node, resources := range usedResourcesOnNodes {
		idleResourcesOnNodes[node], err = SubResourceList(allocatableResourcesOnNodes[node], resources)
		if err != nil {
			klog.Errorf("Failed to calculate idle resources on node <%s>, error: %s.", node, err.Error())
			return
		}
	}

	// filter and count nodes according to idle CPU and memories
	for _, resources := range idleResourcesOnNodes {
		cpu := resources.Cpu()
		for _, phase := range CPUPhases {
			var quantityPhase, _ = resource.ParseQuantity(phase)
			if cpu.Cmp(quantityPhase) == 1 {
				if _, exist := resourcesInfo.NodesWithIdleCPU[phase]; !exist {
					resourcesInfo.NodesWithIdleCPU[phase] = 0
				}
				resourcesInfo.NodesWithIdleCPU[phase]++
			}
		}
		mem := resources.Memory()
		for _, phase := range MEMPhases {
			var quantityPhase, _ = resource.ParseQuantity(phase)
			if mem.Cmp(quantityPhase) == 1 {
				if _, exist := resourcesInfo.NodesWithIdleMem[phase]; !exist {
					resourcesInfo.NodesWithIdleMem[phase] = 0
				}
				resourcesInfo.NodesWithIdleMem[phase]++
			}
		}
	}

	resourcesInfo.Allocatable = CalculateResourcesSum(allocatableResourcesOnNodes)
	resourcesInfo.Used = CalculateResourcesSum(usedResourcesOnNodes)
	resourcesInfo.Idle = CalculateResourcesSum(idleResourcesOnNodes)

	resourcesInfo.Allocatable[v1.ResourceMemory], _ = resource.ParseQuantity(
		strconv.FormatInt(resourcesInfo.Allocatable.Memory().Value()/1000/1000*1000*1000, 10))
	resourcesInfo.Used[v1.ResourceMemory], _ = resource.ParseQuantity(
		strconv.FormatInt(resourcesInfo.Used.Memory().Value()/1000/1000*1000*1000, 10))
	resourcesInfo.Idle[v1.ResourceMemory], _ = resource.ParseQuantity(
		strconv.FormatInt(resourcesInfo.Idle.Memory().Value()/1000/1000*1000*1000, 10))

	resourcesInfo.DeepCopyInto(&newClusterInfo.Spec.Resources)

	if _, err := ci.vcClient.FederationV1alpha1().ClusterInfos().
		Update(context.TODO(), newClusterInfo, metav1.UpdateOptions{}); err != nil {
		klog.Errorf("Failed to update cluster info, error: %s.", err.Error())
		return
	}
}
