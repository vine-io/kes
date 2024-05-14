/*
   Copyright 2024 The kes Authors

   This program is offered under a commercial and under the AGPL license.
   For AGPL licensing, see below.

   AGPL licensing:
   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package app

import (
	"errors"
	"fmt"
	"net/http"
	gpath "path"
	"strings"

	"go.etcd.io/etcd/server/v3/etcdserver/api/v3client"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/managedfields"
	genericapi "k8s.io/apiserver/pkg/endpoints"
	"k8s.io/apiserver/pkg/endpoints/discovery"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storageversion"
	openapibuilder3 "k8s.io/kube-openapi/pkg/builder3"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	openapiutil "k8s.io/kube-openapi/pkg/util"

	generickeserver "github.com/vine-io/kes/pkg/server"

	"github.com/vine-io/kes/pkg/embed"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
)

const (
	// DefaultLegacyAPIPrefix is where the legacy APIs will be located.
	DefaultLegacyAPIPrefix = "/api"

	// APIGroupPrefix is where non-legacy API group will be located.
	APIGroupPrefix = "/apis"
)

func init() {
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

var getOpenAPIDefinitions = func(callback openapicommon.ReferenceCallback) map[string]openapicommon.OpenAPIDefinition {
	return map[string]openapicommon.OpenAPIDefinition{}
}

type App struct {
	Serializer runtime.NegotiatedSerializer

	Handler *generickeserver.APIServerHandler

	// Enable swagger and/or OpenAPI V3 if these configs are non-nil.
	openAPIV3Config *openapicommon.OpenAPIV3Config
}

func NewApp() *App {

	name := "custom-server"
	Serializer := Codecs
	handlerChainBuilder := func(apiHandler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {})
	}
	delegationTarget := &emptyDelegate{}
	apiServerHandler := generickeserver.NewAPIServerHandler(name, Serializer, handlerChainBuilder, delegationTarget.UnprotectedHandler())
	namer := openapinamer.NewDefinitionNamer(Scheme)
	openAPIV3Config := generickeserver.DefaultOpenAPIV3Config(getOpenAPIDefinitions, namer)

	app := &App{
		Serializer:      Serializer,
		Handler:         apiServerHandler,
		openAPIV3Config: openAPIV3Config,
	}

	return app
}

func (app *App) Start(stopc <-chan struct{}) error {
	cfg := embed.NewConfig()

	cfg.UserHandlers = map[string]http.Handler{}
	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		return err
	}

	select {
	case <-etcd.Server.ReadyNotify():
	case <-stopc:
	}

	v3client.New(etcd.Server)

	select {
	case <-stopc:
	}

	return nil
}

// InstallAPIGroup exposes the given api group in the API.
// The <apiGroupInfo> passed into this function shouldn't be used elsewhere as the
// underlying storage will be destroyed on this servers shutdown.
func (app *App) InstallAPIGroup(apiGroupInfo *generickeserver.APIGroupInfo) error {
	return app.InstallAPIGroups(apiGroupInfo)
}

// InstallAPIGroups exposes given api groups in the API.
// The <apiGroupInfos> passed into this function shouldn't be used elsewhere as the
// underlying storage will be destroyed on this servers shutdown.
func (app *App) InstallAPIGroups(apiGroupInfos ...*generickeserver.APIGroupInfo) error {
	for _, apiGroupInfo := range apiGroupInfos {
		if len(apiGroupInfo.PrioritizedVersions) == 0 {
			return fmt.Errorf("no version priority set for %#v", *apiGroupInfo)
		}
		// Do not register empty group or empty version.  Doing so claims /apis/ for the wrong entity to be returned.
		// Catching these here places the error  much closer to its origin
		if len(apiGroupInfo.PrioritizedVersions[0].Group) == 0 {
			return fmt.Errorf("cannot register handler with an empty group for %#v", *apiGroupInfo)
		}
		if len(apiGroupInfo.PrioritizedVersions[0].Version) == 0 {
			return fmt.Errorf("cannot register handler with an empty version for %#v", *apiGroupInfo)
		}
	}

	openAPIModels, err := app.getOpenAPIModels(APIGroupPrefix, apiGroupInfos...)
	if err != nil {
		return fmt.Errorf("unable to get openapi models: %v", err)
	}

	for _, apiGroupInfo := range apiGroupInfos {
		if err := app.installAPIResources(APIGroupPrefix, apiGroupInfo, openAPIModels); err != nil {
			return fmt.Errorf("unable to install api resources: %v", err)
		}

		// setup discovery
		// Install the version handler.
		// Add a handler at /apis/<groupName> to enumerate all versions supported by this group.
		apiVersionsForDiscovery := []metav1.GroupVersionForDiscovery{}
		for _, groupVersion := range apiGroupInfo.PrioritizedVersions {
			// Check the config to make sure that we elide versions that don't have any resources
			if len(apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version]) == 0 {
				continue
			}
			apiVersionsForDiscovery = append(apiVersionsForDiscovery, metav1.GroupVersionForDiscovery{
				GroupVersion: groupVersion.String(),
				Version:      groupVersion.Version,
			})
		}
		preferredVersionForDiscovery := metav1.GroupVersionForDiscovery{
			GroupVersion: apiGroupInfo.PrioritizedVersions[0].String(),
			Version:      apiGroupInfo.PrioritizedVersions[0].Version,
		}
		apiGroup := metav1.APIGroup{
			Name:             apiGroupInfo.PrioritizedVersions[0].Group,
			Versions:         apiVersionsForDiscovery,
			PreferredVersion: preferredVersionForDiscovery,
		}

		//s.DiscoveryGroupManager.AddGroup(apiGroup)
		app.Handler.GoRestfulContainer.Add(discovery.NewAPIGroupHandler(app.Serializer, apiGroup).WebService())
	}

	return nil
}

// installAPIResources is a private method for installing the REST storage backing each api groupversionresource
func (app *App) installAPIResources(apiPrefix string, apiGroupInfo *generickeserver.APIGroupInfo, typeConverter managedfields.TypeConverter) error {
	var resourceInfos []*storageversion.ResourceInfo
	for _, groupVersion := range apiGroupInfo.PrioritizedVersions {
		if len(apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version]) == 0 {
			// klog.Warningf("Skipping API %v because it has no resources.", groupVersion)
			continue
		}

		apiGroupVersion, err := app.getAPIGroupVersion(apiGroupInfo, groupVersion, apiPrefix)
		if err != nil {
			return err
		}
		apiGroupVersion.TypeConverter = typeConverter
		apiGroupVersion.MaxRequestBodyBytes = 1 << 20

		discoveryAPIResources, r, err := apiGroupVersion.InstallREST(app.Handler.GoRestfulContainer)
		_ = discoveryAPIResources

		if err != nil {
			return fmt.Errorf("unable to setup API %v: %v", apiGroupInfo, err)
		}
		resourceInfos = append(resourceInfos, r...)

		// Aggregated discovery only aggregates resources under /apis
		if apiPrefix == APIGroupPrefix {
			//s.AggregatedDiscoveryGroupManager.AddGroupVersion(
			//	groupVersion.Group,
			//	apidiscoveryv2.APIVersionDiscovery{
			//		Freshness: apidiscoveryv2.DiscoveryFreshnessCurrent,
			//		Version:   groupVersion.Version,
			//		Resources: discoveryAPIResources,
			//	},
			//)
		} else {
			// There is only one group version for legacy resources, priority can be defaulted to 0.
			//s.AggregatedLegacyDiscoveryGroupManager.AddGroupVersion(
			//	groupVersion.Group,
			//	apidiscoveryv2.APIVersionDiscovery{
			//		Freshness: apidiscoveryv2.DiscoveryFreshnessCurrent,
			//		Version:   groupVersion.Version,
			//		Resources: discoveryAPIResources,
			//	},
			//)
		}
	}

	//s.RegisterDestroyFunc(apiGroupInfo.destroyStorage)
	//
	//if utilfeature.DefaultFeatureGate.Enabled(features.StorageVersionAPI) &&
	//	utilfeature.DefaultFeatureGate.Enabled(features.APIServerIdentity) {
	//	// API installation happens before we start listening on the handlers,
	//	// therefore it is safe to register ResourceInfos here. The handler will block
	//	// write requests until the storage versions of the targeting resources are updated.
	//	s.StorageVersionManager.AddResourceInfo(resourceInfos...)
	//}

	return nil
}

func (app *App) getAPIGroupVersion(apiGroupInfo *generickeserver.APIGroupInfo, groupVersion schema.GroupVersion, apiPrefix string) (*genericapi.APIGroupVersion, error) {
	storage := make(map[string]rest.Storage)
	for k, v := range apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version] {
		if strings.ToLower(k) != k {
			return nil, fmt.Errorf("resource names must be lowercase only, not %q", k)
		}
		storage[k] = v
	}
	version := app.newAPIGroupVersion(apiGroupInfo, groupVersion)
	version.Root = apiPrefix
	version.Storage = storage
	return version, nil
}

func (app *App) newAPIGroupVersion(apiGroupInfo *generickeserver.APIGroupInfo, groupVersion schema.GroupVersion) *genericapi.APIGroupVersion {

	allServedVersionsByResource := map[string][]string{}
	for version, resourcesInVersion := range apiGroupInfo.VersionedResourcesStorageMap {
		for resource := range resourcesInVersion {
			if len(groupVersion.Group) == 0 {
				allServedVersionsByResource[resource] = append(allServedVersionsByResource[resource], version)
			} else {
				allServedVersionsByResource[resource] = append(allServedVersionsByResource[resource], fmt.Sprintf("%s/%s", groupVersion.Group, version))
			}
		}
	}

	return &genericapi.APIGroupVersion{
		GroupVersion:                groupVersion,
		AllServedVersionsByResource: allServedVersionsByResource,
		MetaGroupVersion:            apiGroupInfo.MetaGroupVersion,

		ParameterCodec:        apiGroupInfo.ParameterCodec,
		Serializer:            apiGroupInfo.NegotiatedSerializer,
		Creater:               apiGroupInfo.Scheme,
		Convertor:             apiGroupInfo.Scheme,
		ConvertabilityChecker: apiGroupInfo.Scheme,
		UnsafeConvertor:       runtime.UnsafeObjectConvertor(apiGroupInfo.Scheme),
		Defaulter:             apiGroupInfo.Scheme,
		Typer:                 apiGroupInfo.Scheme,
		Namer:                 runtime.Namer(meta.NewAccessor()),

		//EquivalentResourceRegistry: app.EquivalentResourceRegistry,

		//Admit:             app.admissionControl,
		//MinRequestTimeout: app.minRequestTimeout,
		//Authorizer:        app.Authorizer,
	}
}

// getOpenAPIModels is a private method for getting the OpenAPI models
func (app *App) getOpenAPIModels(apiPrefix string, apiGroupInfos ...*generickeserver.APIGroupInfo) (managedfields.TypeConverter, error) {
	if app.openAPIV3Config == nil {
		// SSA is GA and requires OpenAPI config to be set
		// to create models.
		return nil, errors.New("OpenAPIV3 config must not be nil")
	}
	pathsToIgnore := openapiutil.NewTrie(app.openAPIV3Config.IgnorePrefixes)
	resourceNames := make([]string, 0)
	for _, apiGroupInfo := range apiGroupInfos {
		groupResources, err := getResourceNamesForGroup(apiPrefix, apiGroupInfo, pathsToIgnore)
		if err != nil {
			return nil, err
		}
		resourceNames = append(resourceNames, groupResources...)
	}

	// Build the openapi definitions for those resources and convert it to proto models
	openAPISpec, err := openapibuilder3.BuildOpenAPIDefinitionsForResources(app.openAPIV3Config, resourceNames...)
	if err != nil {
		return nil, err
	}
	for _, apiGroupInfo := range apiGroupInfos {
		apiGroupInfo.StaticOpenAPISpec = openAPISpec
	}

	typeConverter, err := managedfields.NewTypeConverter(openAPISpec, false)
	if err != nil {
		return nil, err
	}

	return typeConverter, nil
}

// getResourceNamesForGroup is a private method for getting the canonical names for each resource to build in an api group
func getResourceNamesForGroup(apiPrefix string, apiGroupInfo *generickeserver.APIGroupInfo, pathsToIgnore openapiutil.Trie) ([]string, error) {
	// Get the canonical names of every resource we need to build in this api group
	resourceNames := make([]string, 0)
	for _, groupVersion := range apiGroupInfo.PrioritizedVersions {
		for resource, storage := range apiGroupInfo.VersionedResourcesStorageMap[groupVersion.Version] {
			path := gpath.Join(apiPrefix, groupVersion.Group, groupVersion.Version, resource)
			if !pathsToIgnore.HasPrefix(path) {
				kind, err := genericapi.GetResourceKind(groupVersion, storage, apiGroupInfo.Scheme)
				if err != nil {
					return nil, err
				}
				sampleObject, err := apiGroupInfo.Scheme.New(kind)
				if err != nil {
					return nil, err
				}
				name := openapiutil.GetCanonicalTypeName(sampleObject)
				resourceNames = append(resourceNames, name)
			}
		}
	}

	return resourceNames, nil
}
