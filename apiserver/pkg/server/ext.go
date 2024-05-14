/*
Copyright 2017 The Kubernetes Authors.

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

package server

import (
	"net/url"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	restregistry "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	openapicommon "k8s.io/kube-openapi/pkg/common"

	"github.com/vine-io/kes/apiserver/pkg/server/resource"
	"github.com/vine-io/kes/apiserver/pkg/server/resource/resourcestrategy"
	"github.com/vine-io/kes/apiserver/pkg/server/rest"
)

var (
	APIs                = map[schema.GroupVersionResource]rest.StorageProvider{}
	GenericAPIServerFns []func(*genericapiserver.GenericAPIServer) *genericapiserver.GenericAPIServer
)

var (
	ParameterScheme = runtime.NewScheme()
	ParameterCodec  = runtime.NewParameterCodec(ParameterScheme)
)

var (
	EtcdPath             string
	RecommendedConfigFns []func(*genericapiserver.RecommendedConfig) *genericapiserver.RecommendedConfig
	ServerOptionsFns     []func(server *ServerOptions) *ServerOptions
	FlagsFns             []func(fs *pflag.FlagSet) *pflag.FlagSet
)

type ServerOptions = WardleServerOptions

func init() {
	metav1.AddMetaToScheme(ParameterScheme)
}

func BuildAPIGroupInfos(s *runtime.Scheme, g genericregistry.RESTOptionsGetter) ([]*genericapiserver.APIGroupInfo, error) {
	resourcesByGroupVersion := make(map[schema.GroupVersion]sets.String)
	groups := sets.NewString()
	for gvr := range APIs {
		groups.Insert(gvr.Group)
		if resourcesByGroupVersion[gvr.GroupVersion()] == nil {
			resourcesByGroupVersion[gvr.GroupVersion()] = sets.NewString()
		}
		resourcesByGroupVersion[gvr.GroupVersion()].Insert(gvr.Resource)
	}
	apiGroups := []*genericapiserver.APIGroupInfo{}
	for _, group := range groups.List() {
		apis := map[string]map[string]restregistry.Storage{}
		for gvr, storageProviderFunc := range APIs {
			if gvr.Group == group {
				if _, found := apis[gvr.Version]; !found {
					apis[gvr.Version] = map[string]restregistry.Storage{}
				}
				storage, err := storageProviderFunc(s, g)
				if err != nil {
					return nil, err
				}
				apis[gvr.Version][gvr.Resource] = storage
				// add the defaulting function for this version to the scheme
				if _, ok := storage.(resourcestrategy.Defaulter); ok {
					if obj, ok := storage.(runtime.Object); ok {
						s.AddTypeDefaultingFunc(obj, func(obj interface{}) {
							obj.(resourcestrategy.Defaulter).Default()
						})
					}
				}
				if c, ok := storage.(restregistry.Connecter); ok {
					optionsObj, _, _ := c.NewConnectOptions()
					if optionsObj != nil {
						ParameterScheme.AddKnownTypes(gvr.GroupVersion(), optionsObj)
						Scheme.AddKnownTypes(gvr.GroupVersion(), optionsObj)
						if _, ok := optionsObj.(resource.QueryParameterObject); ok {
							if err := ParameterScheme.AddConversionFunc(&url.Values{}, optionsObj, func(src interface{}, dest interface{}, s conversion.Scope) error {
								return dest.(resource.QueryParameterObject).ConvertFromUrlValues(src.(*url.Values))
							}); err != nil {
								return nil, err
							}
						}
					}
				}
			}
		}
		apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(group, Scheme, ParameterCodec, Codecs)
		apiGroupInfo.VersionedResourcesStorageMap = apis
		apiGroups = append(apiGroups, &apiGroupInfo)
	}
	return apiGroups, nil
}

func ApplyGenericAPIServerFns(in *genericapiserver.GenericAPIServer) *genericapiserver.GenericAPIServer {
	for i := range GenericAPIServerFns {
		in = GenericAPIServerFns[i](in)
	}
	return in
}

func ApplyServerOptionsFns(in *ServerOptions) *ServerOptions {
	for i := range ServerOptionsFns {
		in = ServerOptionsFns[i](in)
	}
	return in
}

func ApplyRecommendedConfigFns(in *genericapiserver.RecommendedConfig) *genericapiserver.RecommendedConfig {
	for i := range RecommendedConfigFns {
		in = RecommendedConfigFns[i](in)
	}
	return in
}

func ApplyFlagsFns(fs *pflag.FlagSet) *pflag.FlagSet {
	for i := range FlagsFns {
		fs = FlagsFns[i](fs)
	}
	return fs
}

func SetOpenAPIDefinitions(name, version string, defs openapicommon.GetOpenAPIDefinitions) {
	RecommendedConfigFns = append(RecommendedConfigFns, func(config *genericapiserver.RecommendedConfig) *genericapiserver.RecommendedConfig {
		config.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(defs, openapi.NewDefinitionNamer(Scheme))
		config.OpenAPIV3Config.Info.Title = name
		config.OpenAPIV3Config.Info.Version = version
		return config
	})
}

func getEctdPath() string {
	// TODO: make this configurable
	return "/registry/sample-apiserver"
}
