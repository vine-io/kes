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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/vine-io/kes/apiserver-runtime/sample/pkg/apis/sample/v1alpha1"
	clientset "github.com/vine-io/kes/apiserver-runtime/sample/pkg/generated/clientset/versioned"
	_ "github.com/vine-io/kes/apiserver-runtime/sample/pkg/generated/clientset/versioned/scheme"
)

func main() {
	//	import (
	//	  "k8s.io/client-go/kubernetes"
	//	  clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	//	  aggregatorclientsetscheme "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/scheme"
	//	)
	//
	//	kclientset, _ := kubernetes.NewForConfig(c)
	//	_ = aggregatorclientsetscheme.AddToScheme(clientsetscheme.Scheme)

	cfg, err := clientcmd.BuildConfigFromFlags("https://127.0.0.1:9443", "kubeconfig")
	if err != nil {
		klog.Fatalf("Error building kubeconfig: %s", err.Error())
		return
	}

	//_ = versionedScheme.AddToScheme(scheme)

	//kubernetes.NewForConfig(cfg)
	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error building clientset: %s", err.Error())
		return
	}

	//if err = v1alpha1.AddToScheme(scheme); err != nil {
	//	klog.Fatalf("Error building scheme: %s", err.Error())
	//}
	//if err = clientgoscheme.AddToScheme(scheme); err != nil {
	//	klog.Fatalf("Error building scheme: %s", err.Error())
	//}

	ctx := context.Background()
	fischers := &v1alpha1.Fischer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Fischer",
			APIVersion: "sample.k8s.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello",
		},
		DisallowedFlunders: []string{"a", "b", "c"},
	}
	fischer, _ := client.SampleV1alpha1().Fischers().Create(ctx, fischers, metav1.CreateOptions{})
	fischer, _ = client.SampleV1alpha1().Fischers().Get(ctx, "hello", metav1.GetOptions{})
	fischer.DisallowedFlunders = fischers.DisallowedFlunders
	fischers, err = client.SampleV1alpha1().Fischers().Update(ctx, fischer, metav1.UpdateOptions{})
	if err != nil {
		klog.Fatalf("Error creating fischer: %s", err.Error())
		return
	}

	//client.SampleV1alpha1().Fischers().Delete(ctx, "hello", metav1.DeleteOptions{})
}
