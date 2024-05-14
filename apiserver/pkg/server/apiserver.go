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
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"

	"github.com/vine-io/kes/apiserver/pkg/etcd"
)

var (
	// Scheme defines methods for serializing and deserializing API objects.
	Scheme = runtime.NewScheme()
	// Codecs provides methods for retrieving codecs and serializers for specific
	// versions and content types.
	Codecs = serializer.NewCodecFactory(Scheme)
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

// WardleServer contains state for a Kubernetes cluster master/api server.
type WardleServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer

	embedEtcd *etcd.Etcd
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
	genericServer = ApplyGenericAPIServerFns(genericServer)

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
		GenericAPIServer: genericServer,
		embedEtcd:        embedEtcd,
	}

	// Add new APIs through inserting into APIs
	apiGroups, err := BuildAPIGroupInfos(Scheme, c.GenericConfig.RESTOptionsGetter)
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
