/*
Copyright 2016 The Kubernetes Authors.

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
	"strings"

	"github.com/go-logr/zapr"
	"github.com/vine-io/kes/apiserver/pkg/apis/sample/v1alpha1"
	"github.com/vine-io/kes/apiserver/pkg/etcd"
	"github.com/vine-io/kes/apiserver/pkg/server/resource"
	"github.com/vine-io/kes/apiserver/pkg/server/resource/resourcerest"
	"github.com/vine-io/kes/apiserver/pkg/server/resource/resourcestrategy"
	"github.com/vine-io/kes/apiserver/pkg/server/rest"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/version"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	restregistry "k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

var (
	ParameterScheme = runtime.NewScheme()
	ParameterCodec  = runtime.NewParameterCodec(ParameterScheme)
)

func getEctdPath() string {
	// TODO: make this configurable
	return "/registry/sample-apiserver"
}

func init() {
	metav1.AddMetaToScheme(ParameterScheme)

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(Scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	Scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)
}

// ExtraConfig holds custom apiserver config
type ExtraConfig struct {
	// Place you custom config here.
}

// Config defines the config for the apiserver
type Config struct {
	GenericConfig *genericapiserver.RecommendedConfig
	ExtraConfig   ExtraConfig
}

// Complete fills in any fields not set that are required to have valid data. It's mutating the receiver.
func (cfg *Config) Complete() CompletedConfig {
	c := completedConfig{
		cfg.GenericConfig.Complete(),
		&cfg.ExtraConfig,
	}

	c.GenericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	return CompletedConfig{&c}
}

// CompletedConfig embeds a private pointer that cannot be instantiated outside of this package.
type CompletedConfig struct {
	*completedConfig
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	ExtraConfig   *ExtraConfig
}

// New returns a new instance of WardleServer from the given config.
func (c completedConfig) New() (*WardleServer, error) {
	genericServer, err := c.GenericConfig.New("sample-apiserver", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	// change: apiserver-runtime
	// genericServer = ApplyGenericAPIServerFns(genericServer)

	etcdCfg := etcd.NewConfig()
	zapLogger := etcdCfg.GetLogger()
	if zapLogger == nil {
		zapLogger = zap.NewExample()
	}
	logr := zapr.NewLogger(zapLogger)
	klog.SetLogger(logr)

	etcdCfg.Dir = "_output"
	embedEtcd, err := etcd.StartEtcd(etcdCfg)
	if err != nil {
		return nil, err
	}
	<-embedEtcd.Server.ReadyNotify()

	s := &WardleServer{
		APIs:                 map[schema.GroupVersionResource]rest.StorageProvider{},
		GenericAPIServer:     genericServer,
		embedEtcd:            embedEtcd,
		errs:                 []error{},
		storageProvider:      map[schema.GroupResource]*singletonProvider{},
		groupVersions:        map[schema.GroupVersion]bool{},
		orderedGroupVersions: []schema.GroupVersion{},
		schemes:              []*runtime.Scheme{},
		//schemeBuilder:       ,
	}

	s.WithResource(&v1alpha1.Fischer{})
	s.WithResource(&v1alpha1.Flunder{})

	//versions := s.orderedGroupVersions
	s.schemes = append(s.schemes, Scheme)
	s.schemeBuilder.Register(
		func(scheme *runtime.Scheme) error {
			groupVersions := make(map[string]sets.Set[string])
			for gvr := range s.APIs {
				if groupVersions[gvr.Group] == nil {
					groupVersions[gvr.Group] = sets.New[string]()
				}
				groupVersions[gvr.Group].Insert(gvr.Version)
			}
			for g, versions := range groupVersions {
				gvs := []schema.GroupVersion{}
				for v := range versions {
					gvs = append(gvs, schema.GroupVersion{
						Group:   g,
						Version: v,
					})
				}
				err := scheme.SetVersionPriority(gvs...)
				if err != nil {
					return err
				}
			}
			for i := range s.orderedGroupVersions {
				metav1.AddToGroupVersion(scheme, s.orderedGroupVersions[i])
			}
			return nil
		},
	)
	for i := range s.schemes {
		if err := s.schemeBuilder.AddToScheme(s.schemes[i]); err != nil {
			s.errs = append(s.errs, err)
		}
	}

	// Add new APIs through inserting into APIs
	apiGroups, err := s.BuildAPIGroupInfos(Scheme, c.GenericConfig.RESTOptionsGetter, s.APIs)
	if err != nil {
		return nil, err
	}
	for _, apiGroup := range apiGroups {
		if err := s.GenericAPIServer.InstallAPIGroup(apiGroup); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// WardleServer contains state for a Kubernetes cluster master/api server.
type WardleServer struct {
	APIs             map[schema.GroupVersionResource]rest.StorageProvider
	GenericAPIServer *genericapiserver.GenericAPIServer

	embedEtcd *etcd.Etcd

	errs                 []error
	storageProvider      map[schema.GroupResource]*singletonProvider
	groupVersions        map[schema.GroupVersion]bool
	orderedGroupVersions []schema.GroupVersion
	schemes              []*runtime.Scheme
	schemeBuilder        runtime.SchemeBuilder
}

func (ws *WardleServer) BuildAPIGroupInfos(s *runtime.Scheme, g genericregistry.RESTOptionsGetter,
	APIs map[schema.GroupVersionResource]rest.StorageProvider) ([]*genericapiserver.APIGroupInfo, error) {
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

// WithResource registers the resource with the apiserver.
//
// If no versions of this GroupResource have already been registered, a new default handler will be registered.
// If the object implements rest.Getter, rest.Updater or rest.Creator then the provided object itself will be
// used as the rest handler for the resource type.
//
// If no versions of this GroupResource have already been registered and the object does NOT implement the rest
// interfaces, then a new etcd backed storage will be created for the object and used as the handler.
// The storage will use a DefaultStrategy, which delegates functions to the object if the object implements
// interfaces defined in the "apiserver-runtime/pkg/builder/rest" package.  Otherwise it will provide a default
// behavior.
//
// WithResource will automatically register the "status" subresource if the object implements the
// resource.StatusGetSetter interface.
//
// WithResource will automatically register version-specific defaulting for this GroupVersionResource
// if the object implements the resource.Defaulter interface.
//
// WithResource automatically adds the object and its list type to the known types.  If the object also declares itself
// as the storage version, the object and its list type will be added as storage versions to the SchemeBuilder as well.
// The storage version is the version accepted by the handler.
//
// If another version of the object's GroupResource has already been registered, then the resource will use the
// handler already registered for that version of the GroupResource.  Objects for this version will be converted
// to the object version which the handler accepts before the handler is invoked.
func (ws *WardleServer) WithResource(obj resource.Object) *WardleServer {
	gvr := obj.GetGroupVersionResource()
	ws.schemeBuilder.Register(resource.AddToScheme(obj))

	// reuse the storage if this resource has already been registered
	if s, found := ws.storageProvider[gvr.GroupResource()]; found {
		_ = ws.forGroupVersionResource(gvr, s.Get)
		return ws
	}

	var parentStorageProvider rest.StorageProvider

	defer func() {
		// automatically create status subresource if the object implements the status interface
		ws.withSubResourceIfExists(obj, parentStorageProvider)
	}()

	// If the type implements it's own storage, then use that
	switch s := obj.(type) {
	case resourcerest.Creator, resourcerest.Updater, resourcerest.Getter, resourcerest.Lister:
		parentStorageProvider = rest.StaticHandlerProvider{Storage: s.(restregistry.Storage)}.Get
	default:
		parentStorageProvider = rest.New(obj)
	}

	_ = ws.forGroupVersionResource(gvr, parentStorageProvider)

	return ws
}

// WithResourceAndStrategy registers the resource with the apiserver creating a new etcd backed storage
// for the GroupResource using the provided strategy.  In most cases callers should instead use WithResource
// and implement the interfaces defined in "apiserver-runtime/pkg/builder/rest" to control the Strategy.
//
// Note: WithResourceAndHandler should never be called after the GroupResource has already been registered with
// another version.
func (ws *WardleServer) WithResourceAndStrategy(obj resource.Object, strategy rest.Strategy) *WardleServer {
	gvr := obj.GetGroupVersionResource()
	ws.schemeBuilder.Register(resource.AddToScheme(obj))

	parentStorageProvider := rest.NewWithStrategy(obj, strategy)
	_ = ws.forGroupVersionResource(gvr, parentStorageProvider)

	// automatically create status subresource if the object implements the status interface

	defer func() {
		// automatically create status subresource if the object implements the status interface
		ws.withSubResourceIfExists(obj, parentStorageProvider)
	}()
	return ws
}

// WithResourceAndHandler registers a request handler for the resource rather than the default
// etcd backed storage.
//
// Note: WithResourceAndHandler should never be called after the GroupResource has already been registered with
// another version.
//
// Note: WithResourceAndHandler will NOT register the "status" subresource for the resource object.
func (ws *WardleServer) WithResourceAndHandler(obj resource.Object, sp rest.StorageProvider) *WardleServer {
	gvr := obj.GetGroupVersionResource()
	ws.schemeBuilder.Register(resource.AddToScheme(obj))
	defer func() {
		// automatically create status subresource if the object implements the status interface
		ws.withSubResourceIfExists(obj, sp)
	}()
	return ws.forGroupVersionResource(gvr, sp)
}

// WithResourceAndStorage registers the resource with the apiserver, applying fn to the storage for the resource
// before completing it.
//
// May be used to change low-level storage configuration or swap out the storage backend to something other than
// etcd.
//
// Note: WithResourceAndHandler should never be called after the GroupResource has already been registered with
// another version.
func (ws *WardleServer) WithResourceAndStorage(obj resource.Object, fn rest.StoreFn) *WardleServer {
	gvr := obj.GetGroupVersionResource()
	ws.schemeBuilder.Register(resource.AddToScheme(obj))
	sp := rest.NewWithFn(obj, fn)
	defer func() {
		// automatically create status subresource if the object implements the status interface
		ws.withSubResourceIfExists(obj, sp)
	}()
	return ws.forGroupVersionResource(gvr, sp)
}

// forGroupVersionResource manually registers storage for a specific resource.
func (ws *WardleServer) forGroupVersionResource(
	gvr schema.GroupVersionResource, sp rest.StorageProvider) *WardleServer {
	// register the group version
	ws.withGroupVersions(gvr.GroupVersion())

	// TODO: make sure folks don't register multiple storageProvider instance for the same group-resource
	// don't replace the existing instance otherwise it will chain wrapped singletonProviders when
	// fetching from the map before calling this function
	if _, found := ws.storageProvider[gvr.GroupResource()]; !found {
		ws.storageProvider[gvr.GroupResource()] = &singletonProvider{Provider: sp}
	}
	// add the API with its storageProvider
	ws.APIs[gvr] = sp
	return ws
}

// forGroupVersionSubResource manually registers storageProvider for a specific subresource.
func (ws *WardleServer) forGroupVersionSubResource(
	gvr schema.GroupVersionResource, parentProvider rest.StorageProvider, subResourceProvider rest.StorageProvider) {
	isSubResource := strings.Contains(gvr.Resource, "/")
	if !isSubResource {
		klog.Fatalf("Expected status subresource but received %v/%v/%v", gvr.Group, gvr.Version, gvr.Resource)
	}

	// add the API with its storageProvider for subresource
	ws.APIs[gvr] = (&subResourceStorageProvider{
		subResourceGVR:             gvr,
		parentStorageProvider:      parentProvider,
		subResourceStorageProvider: subResourceProvider,
	}).Get
}

// WithSchemeInstallers registers functions to install resource types into the Scheme.
func (ws *WardleServer) withGroupVersions(versions ...schema.GroupVersion) *WardleServer {
	if ws.groupVersions == nil {
		ws.groupVersions = map[schema.GroupVersion]bool{}
	}
	for _, gv := range versions {
		if _, found := ws.groupVersions[gv]; found {
			continue
		}
		ws.groupVersions[gv] = true
		ws.orderedGroupVersions = append(ws.orderedGroupVersions, gv)
	}
	return ws
}

func (ws *WardleServer) withSubResourceIfExists(obj resource.Object, parentStorageProvider rest.StorageProvider) {
	parentGVR := obj.GetGroupVersionResource()
	// automatically create status subresource if the object implements the status interface
	if _, ok := obj.(resource.ObjectWithStatusSubResource); ok {
		statusGVR := parentGVR.GroupVersion().WithResource(parentGVR.Resource + "/status")
		ws.forGroupVersionSubResource(statusGVR, parentStorageProvider, nil)
	}
	if _, ok := obj.(resource.ObjectWithScaleSubResource); ok {
		subResourceGVR := parentGVR.GroupVersion().WithResource(parentGVR.Resource + "/scale")
		ws.forGroupVersionSubResource(subResourceGVR, parentStorageProvider, nil)
	}
	if sgs, ok := obj.(resource.ObjectWithArbitrarySubResource); ok {
		for _, sub := range sgs.GetArbitrarySubResources() {
			sub := sub
			subResourceGVR := parentGVR.GroupVersion().WithResource(parentGVR.Resource + "/" + sub.SubResourceName())
			ws.forGroupVersionSubResource(subResourceGVR, parentStorageProvider, rest.ParentStaticHandlerProvider{
				Storage:        sub,
				ParentProvider: parentStorageProvider,
			}.Get)
		}
	}
}
