/*
Copyright 2020 The Kubernetes Authors.

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

package resource_test

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"

"github.com/vine-io/kes/apiserver-runtime/pkg/builder"
"github.com/vine-io/kes/apiserver-runtime/pkg/builder/resource/resourcerest"
)

func ExampleObject_withHandler() {
	// register this resource using the object itself to handle the requests and only exposes
	// endpoints that are implemented by the object (create, update, patch).
	// https://APISERVER_HOST:APISERVER_PORT/apis/sample.k8s.com/v1alpha1/namespaces/NAMESPACE/examples/NAME
	builder.APIServer.WithResource(&ExampleResourceWithHandler{})
}

// ExampleResourceWithHandler defines a resource and implements its request handler functions
type ExampleResourceWithHandler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
}

// Create is invoked when creating a resource -- e.g. for POST calls
func (e ExampleResourceWithHandler) Create(
	ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (
	runtime.Object, error) {
	panic("implement me")
}

// Update is invoked when updating a resource -- e.g. for PUT and PATCH calls
func (e ExampleResourceWithHandler) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	panic("implement me")
}

// ExampleResourceWithHandlerList contains a list of ExampleResourceWithHandler objects
type ExampleResourceWithHandlerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ExampleResourceWithHandler `json:"items" protobuf:"bytes,2,rep,name=items"`
}

var _ resourcerest.CreaterUpdater = &ExampleResourceWithHandler{}

// ExampleResourceWithHandler is required by apimachinery and implemented by deepcopy-gen
func (e ExampleResourceWithHandler) DeepCopyObject() runtime.Object {
	// generated by deepcopy-gen
	panic("implement me")
}

// GetObjectMeta returns the ObjectMeta for the object
func (e ExampleResourceWithHandler) GetObjectMeta() *metav1.ObjectMeta {
	return &e.ObjectMeta
}

// NamespaceScoped returns true to register ExampleResource as a namespaced resource
func (e ExampleResourceWithHandler) NamespaceScoped() bool {
	return true
}

// New returns a new instance of the object for this resource.
func (e ExampleResourceWithHandler) New() runtime.Object {
	return &ExampleResource{}
}

// NewList returns a new instance of the list object for this resource.
func (e ExampleResourceWithHandler) NewList() runtime.Object {
	return &ExampleResourceList{}
}

// GetGroupVersionResource returns the GroupVersionResource for this type.
func (e ExampleResourceWithHandler) GetGroupVersionResource() schema.GroupVersionResource {
	return SchemeGroupVersion.WithResource("examplewithhandlers")
}

// IsStorageVersion returns true for the resource version used as the storage version.
func (e ExampleResourceWithHandler) IsStorageVersion() bool {
	return true
}

// DeepCopyObject is required by apimachinery and generated by deepcopy-gen.
func (e *ExampleResourceWithHandlerList) DeepCopyObject() runtime.Object {
	// generated by deepcopy-gen
	return e
}
