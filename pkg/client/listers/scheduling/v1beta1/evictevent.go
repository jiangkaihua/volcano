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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	v1beta1 "volcano.sh/volcano/pkg/apis/scheduling/v1beta1"
)

// EvictEventLister helps list EvictEvents.
// All objects returned here must be treated as read-only.
type EvictEventLister interface {
	// List lists all EvictEvents in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.EvictEvent, err error)
	// EvictEvents returns an object that can list and get EvictEvents.
	EvictEvents(namespace string) EvictEventNamespaceLister
	EvictEventListerExpansion
}

// evictEventLister implements the EvictEventLister interface.
type evictEventLister struct {
	indexer cache.Indexer
}

// NewEvictEventLister returns a new EvictEventLister.
func NewEvictEventLister(indexer cache.Indexer) EvictEventLister {
	return &evictEventLister{indexer: indexer}
}

// List lists all EvictEvents in the indexer.
func (s *evictEventLister) List(selector labels.Selector) (ret []*v1beta1.EvictEvent, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.EvictEvent))
	})
	return ret, err
}

// EvictEvents returns an object that can list and get EvictEvents.
func (s *evictEventLister) EvictEvents(namespace string) EvictEventNamespaceLister {
	return evictEventNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// EvictEventNamespaceLister helps list and get EvictEvents.
// All objects returned here must be treated as read-only.
type EvictEventNamespaceLister interface {
	// List lists all EvictEvents in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1beta1.EvictEvent, err error)
	// Get retrieves the EvictEvent from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1beta1.EvictEvent, error)
	EvictEventNamespaceListerExpansion
}

// evictEventNamespaceLister implements the EvictEventNamespaceLister
// interface.
type evictEventNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all EvictEvents in the indexer for a given namespace.
func (s evictEventNamespaceLister) List(selector labels.Selector) (ret []*v1beta1.EvictEvent, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1beta1.EvictEvent))
	})
	return ret, err
}

// Get retrieves the EvictEvent from the indexer for a given namespace and name.
func (s evictEventNamespaceLister) Get(name string) (*v1beta1.EvictEvent, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1beta1.Resource("evictevent"), name)
	}
	return obj.(*v1beta1.EvictEvent), nil
}
