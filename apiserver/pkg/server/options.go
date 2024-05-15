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
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"
	"github.com/vine-io/kes/apiserver/pkg/apis/sample/v1alpha1"
	generatedOpenapi "github.com/vine-io/kes/apiserver/pkg/generated/openapi"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// change: apiserver-runtime
//const defaultEtcdPathPrefix = "/registry/wardle.example.com"

// WardleServerOptions contains state for master/api server
type WardleServerOptions struct {
	RecommendedOptions *RecommendedOptions

	StdOut io.Writer
	StdErr io.Writer
}

// NewWardleServerOptions returns a new WardleServerOptions
func NewWardleServerOptions(out, errOut io.Writer) *WardleServerOptions {
	// change: apiserver-runtime

	o := &WardleServerOptions{

		StdOut: out,
		StdErr: errOut,
	}

	versions := []schema.GroupVersion{v1alpha1.SchemeGroupVersion}
	o.RecommendedOptions = NewRecommendedOptions(
		getEctdPath(),
		Codecs.LegacyCodec(versions...),
	)

	o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = schema.GroupVersions(versions)
	//o.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}

	//o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = runtime.NewMultiGroupVersioner(v1alpha1.SchemeGroupVersion, schema.GroupKind{Group: v1alpha1.GroupName})
	o.RecommendedOptions.Admission = nil
	o.RecommendedOptions.CoreAPI = nil
	//o.RecommendedOptions.Authentication = nil
	//o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true

	return o
}

// Validate validates WardleServerOptions
func (o WardleServerOptions) Validate(args []string) error {
	errors := make([]error, 0)
	errors = append(errors, o.RecommendedOptions.Validate()...)
	return utilerrors.NewAggregate(errors)
}

// Complete fills in fields required to have valid data
func (o *WardleServerOptions) Complete() error {

	return nil
}

// Config returns config for the api server given WardleServerOptions
func (o *WardleServerOptions) Config() (*Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	// change: allow etcd options to be nil
	// TODO: this should be reverted after rebasing sample-apiserver onto https://github.com/kubernetes/kubernetes/pull/101106
	if o.RecommendedOptions.Etcd != nil {
		//o.RecommendedOptions.Etcd.StorageConfig.Paging = utilfeature.DefaultFeatureGate.Enabled(features.APIListChunking)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(Codecs)

	//o.RecommendedOptions.CoreAPI = nil
	//o.RecommendedOptions.Admission = nil
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	name, version, defs := "sample", "v1.0.0", generatedOpenapi.GetOpenAPIDefinitions
	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(defs, openapi.NewDefinitionNamer(Scheme))
	serverConfig.OpenAPIV3Config.Info.Title = name
	serverConfig.OpenAPIV3Config.Info.Version = version

	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(defs, openapi.NewDefinitionNamer(Scheme))
	serverConfig.OpenAPIConfig.Info.Title = name
	serverConfig.OpenAPIConfig.Info.Version = version

	//serverConfig = ApplyRecommendedConfigFns(serverConfig)

	config := &Config{
		GenericConfig: serverConfig,
		ExtraConfig:   ExtraConfig{},
	}
	return config, nil
}

// RunWardleServer starts a new WardleServer given WardleServerOptions
func (o WardleServerOptions) RunWardleServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	wardleServer, err := config.Complete().New()
	if err != nil {
		return err
	}

	wardleServer.GenericAPIServer.AddPostStartHookOrDie("start-sample-server-informers", func(context genericapiserver.PostStartHookContext) error {
		if config.GenericConfig.SharedInformerFactory != nil {
			config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		}
		return nil
	})

	return wardleServer.GenericAPIServer.PrepareRun().Run(stopCh)
}

// NewCommandStartWardleServer provides a CLI handler for 'start master' command
func NewCommandStartWardleServer(o *WardleServerOptions, stopCh <-chan struct{}) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Launch a wardle API server",
		Long:  "Launch a wardle API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunWardleServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)

	return cmd
}
