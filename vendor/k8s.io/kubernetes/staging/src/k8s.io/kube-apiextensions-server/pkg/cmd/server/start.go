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
	"fmt"
	"io"
	"net"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/kube-apiextensions-server/pkg/apis/apiextensions/v1alpha1"
	"k8s.io/kube-apiextensions-server/pkg/apiserver"
)

const defaultEtcdPathPrefix = "/registry/apiextensions.kubernetes.io"

type CustomResourceDefinitionsServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	StdOut io.Writer
	StdErr io.Writer
}

func NewCustomResourceDefinitionsServerOptions(out, errOut io.Writer) *CustomResourceDefinitionsServerOptions {
	o := &CustomResourceDefinitionsServerOptions{
		RecommendedOptions: genericoptions.NewRecommendedOptions(defaultEtcdPathPrefix, apiserver.Scheme, apiserver.Codecs.LegacyCodec(v1alpha1.SchemeGroupVersion)),

		StdOut: out,
		StdErr: errOut,
	}

	return o
}

func NewCommandStartCustomResourceDefinitionsServer(out, errOut io.Writer, stopCh <-chan struct{}) *cobra.Command {
	o := NewCustomResourceDefinitionsServerOptions(out, errOut)

	cmd := &cobra.Command{
		Short: "Launch an API extensions API server",
		Long:  "Launch an API extensions API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunCustomResourceDefinitionsServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)

	return cmd
}

func (o CustomResourceDefinitionsServerOptions) Validate(args []string) error {
	return nil
}

func (o *CustomResourceDefinitionsServerOptions) Complete() error {
	return nil
}

func (o CustomResourceDefinitionsServerOptions) Config() (*apiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewConfig(apiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}

	customResourceDefinitionRESTOptionsGetter := apiserver.CustomResourceDefinitionRESTOptionsGetter{
		StorageConfig:           o.RecommendedOptions.Etcd.StorageConfig,
		StoragePrefix:           o.RecommendedOptions.Etcd.StorageConfig.Prefix,
		EnableWatchCache:        o.RecommendedOptions.Etcd.EnableWatchCache,
		DefaultWatchCacheSize:   o.RecommendedOptions.Etcd.DefaultWatchCacheSize,
		EnableGarbageCollection: o.RecommendedOptions.Etcd.EnableGarbageCollection,
		DeleteCollectionWorkers: o.RecommendedOptions.Etcd.DeleteCollectionWorkers,
	}
	customResourceDefinitionRESTOptionsGetter.StorageConfig.Codec = unstructured.UnstructuredJSONScheme
	customResourceDefinitionRESTOptionsGetter.StorageConfig.Copier = apiserver.UnstructuredCopier{}

	config := &apiserver.Config{
		GenericConfig:                             serverConfig,
		CustomResourceDefinitionRESTOptionsGetter: customResourceDefinitionRESTOptionsGetter,
	}
	return config, nil
}

func (o CustomResourceDefinitionsServerOptions) RunCustomResourceDefinitionsServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New(genericapiserver.EmptyDelegate)
	if err != nil {
		return err
	}
	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
