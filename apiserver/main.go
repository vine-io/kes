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

package main

import (
	"os"

	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/vine-io/kes/apiserver/pkg/apis/sample/v1alpha1"
	"github.com/vine-io/kes/apiserver/pkg/generated/openapi"
	"github.com/vine-io/kes/apiserver/pkg/server"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	o := server.NewWardleServerOptions(os.Stdout, os.Stderr)
	err := o.WithOpenAPIDefinitions("sample", "v1.0.0", openapi.GetOpenAPIDefinitions).
		WithResource(&v1alpha1.Flunder{}). // namespaced resource
		WithResource(&v1alpha1.Fischer{}). // non-namespaced resource
		// WithRes(&v1alpha1.Fortune{}). // resource with custom rest.Storage implementation
		//WithLocalDebugExtension().
		Execute(genericapiserver.SetupSignalHandler())

	if err != nil {
		klog.Fatal(err)
	}
}
