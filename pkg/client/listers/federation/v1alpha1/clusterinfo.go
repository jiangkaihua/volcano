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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1alpha1 "volcano.sh/volcano/pkg/apis/federation/v1alpha1"
)

// ClusterInfoLister helps list ClusterInfos.
// All objects returned here must be treated as read-only.
type ClusterInfoLister interface {
	// List lists all ClusterInfos in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.ClusterInfo, err error)
	// Get retrieves the ClusterInfo from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.ClusterInfo, error)
	ClusterInfoListerExpansion
}

// clusterInfoLister implements the ClusterInfoLister interface.
type clusterInfoLister struct {
	indexer cache.Indexer
}

// NewClusterInfoLister returns a new ClusterInfoLister.
func NewClusterInfoLister(indexer cache.Indexer) ClusterInfoLister {
	return &clusterInfoLister{indexer: indexer}
}

// List lists all ClusterInfos in the indexer.
func (s *clusterInfoLister) List(selector labels.Selector) (ret []*v1alpha1.ClusterInfo, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.ClusterInfo))
	})
	return ret, err
}

// Get retrieves the ClusterInfo from the index for a given name.
func (s *clusterInfoLister) Get(name string) (*v1alpha1.ClusterInfo, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("clusterinfo"), name)
	}
	return obj.(*v1alpha1.ClusterInfo), nil
}
