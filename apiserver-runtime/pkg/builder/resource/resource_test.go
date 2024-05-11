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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

"github.com/vine-io/kes/apiserver-runtime/pkg/builder"
"github.com/vine-io/kes/apiserver-runtime/pkg/builder/resource"
)

func ExampleObject() {
	// register this resource using the default etcd storage under
	// https://APISERVER_HOST:APISERVER_PORT/apis/sample.k8s.com/v1alpha1/namespaces/NAMESPACE/examples/NAME
	builder.APIServer.WithResource(&ExampleResource{})
}

var (
	// register the APIs in this package under the sample.k8s.com group and v1alpha1 version
	SchemeGroupVersion = schema.GroupVersion{Group: "sample.k8s.com", Version: "v1alpha1"}
	// AddToScheme is required for generated clients to compile
	// nolint:unused
	AddToScheme = resource.AddToScheme(&ExampleResource{})
)

type ExampleResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
}

type ExampleResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ExampleResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// DeepCopyObject is required by apimachinery and implemented by deepcopy-gen
func (e ExampleResource) DeepCopyObject() runtime.Object {
	// generated by deepcopy-gen
	panic("implement me")
}

// GetObjectMeta returns the ObjectMeta for the object
func (e ExampleResource) GetObjectMeta() *metav1.ObjectMeta {
	return &e.ObjectMeta
}

// NamespaceScoped returns true to register ExampleResource as a namespaced resource
func (e ExampleResource) NamespaceScoped() bool {
	return true
}

// New returns a new instance of the object for this resource.
func (e ExampleResource) New() runtime.Object {
	return &ExampleResource{}
}

// NewList returns a new instance of the list object for this resource.
func (e ExampleResource) NewList() runtime.Object {
	return &ExampleResourceList{}
}

// GetGroupVersionResource returns the GroupVersionResource for this type.
func (e ExampleResource) GetGroupVersionResource() schema.GroupVersionResource {
	return SchemeGroupVersion.WithResource("exampleresources")
}

// IsStorageVersion returns true for the resource version used as the storage version.
func (e ExampleResource) IsStorageVersion() bool {
	return true
}

// DeepCopyObject is required by apimachinery and generated by deepcopy-gen.
func (e *ExampleResourceList) DeepCopyObject() runtime.Object {
	// generated by deepcopy-gen
	return e
}
