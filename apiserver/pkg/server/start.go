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
	"flag"
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// change: apiserver-runtime
//const defaultEtcdPathPrefix = "/registry/wardle.example.com"

// WardleServerOptions contains state for master/api server
type WardleServerOptions struct {
	RecommendedOptions *RecommendedOptions

	errs                 []error
	storageProvider      map[schema.GroupResource]*singletonProvider
	groupVersions        map[schema.GroupVersion]bool
	orderedGroupVersions []schema.GroupVersion
	schemes              []*runtime.Scheme
	schemeBuilder        runtime.SchemeBuilder

	StdOut io.Writer
	StdErr io.Writer
}

// NewWardleServerOptions returns a new WardleServerOptions
func NewWardleServerOptions(out, errOut io.Writer) *WardleServerOptions {
	// change: apiserver-runtime

	o := &WardleServerOptions{
		errs:                 make([]error, 0),
		storageProvider:      map[schema.GroupResource]*singletonProvider{},
		groupVersions:        map[schema.GroupVersion]bool{},
		orderedGroupVersions: []schema.GroupVersion{},
		schemes:              make([]*runtime.Scheme, 0),
		schemeBuilder:        make(runtime.SchemeBuilder, 0),

		StdOut: out,
		StdErr: errOut,
	}

	return o
}

// Validate validates WardleServerOptions
func (o WardleServerOptions) Validate(args []string) error {
	errors := o.errs
	errors = append(errors, o.RecommendedOptions.Validate()...)
	return utilerrors.NewAggregate(errors)
}

// Complete fills in fields required to have valid data
func (o *WardleServerOptions) Complete() error {

	ApplyServerOptionsFns(o)

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

	o.RecommendedOptions.CoreAPI = nil
	o.RecommendedOptions.Admission = nil
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	serverConfig = ApplyRecommendedConfigFns(serverConfig)

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

// Execute provides a CLI handler for 'start master' command
func (o *WardleServerOptions) Execute(stopCh <-chan struct{}) error {
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

	versions := o.orderedGroupVersions
	o.schemes = append(o.schemes, Scheme)
	o.schemeBuilder.Register(
		func(scheme *runtime.Scheme) error {
			groupVersions := make(map[string]sets.Set[string])
			for gvr := range APIs {
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
			for i := range o.orderedGroupVersions {
				metav1.AddToGroupVersion(scheme, o.orderedGroupVersions[i])
			}
			return nil
		},
	)
	for i := range o.schemes {
		if err := o.schemeBuilder.AddToScheme(o.schemes[i]); err != nil {
			o.errs = append(o.errs, err)
		}
	}

	o.RecommendedOptions = NewRecommendedOptions(
		getEctdPath(),
		Codecs.LegacyCodec(versions...),
	)

	o.RecommendedOptions.Etcd.StorageConfig.EncodeVersioner = schema.GroupVersions(versions)
	//o.RecommendedOptions.Etcd.StorageConfig.Transport.ServerList = []string{"http://127.0.0.1:2379"}

	flags := cmd.Flags()
	ApplyFlagsFns(flags)
	flags.AddGoFlagSet(flag.CommandLine)

	o.RecommendedOptions.AddFlags(flags)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)

	return cmd.Execute()
}
