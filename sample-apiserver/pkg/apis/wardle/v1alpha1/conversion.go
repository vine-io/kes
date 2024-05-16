/*
Copyright 2018 The Kubernetes Authors.

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

package v1alpha1

import (
	"github.com/vine-io/kes/sample-apiserver/pkg/apis/wardle"
	"k8s.io/apimachinery/pkg/conversion"
)

// Convert_v1alpha1_FlunderSpec_To_wardle_FlunderSpec is an autogenerated conversion function.
func Convert_v1alpha1_FlunderSpec_To_wardle_FlunderSpec(in *FlunderSpec, out *wardle.FlunderSpec, s conversion.Scope) error {
	if in.ReferenceType != nil {
		// assume that ReferenceType is defaulted
		switch *in.ReferenceType {
		case FlunderReferenceType:
			out.ReferenceType = wardle.FlunderReferenceType
			out.FlunderReference = in.Reference
		case FischerReferenceType:
			out.ReferenceType = wardle.FischerReferenceType
			out.FischerReference = in.Reference
		}
	}

	return nil
}

// Convert_wardle_FlunderSpec_To_v1alpha1_FlunderSpec is an autogenerated conversion function.
func Convert_wardle_FlunderSpec_To_v1alpha1_FlunderSpec(in *wardle.FlunderSpec, out *FlunderSpec, s conversion.Scope) error {
	switch in.ReferenceType {
	case wardle.FlunderReferenceType:
		t := FlunderReferenceType
		out.ReferenceType = &t
		out.Reference = in.FlunderReference
	case wardle.FischerReferenceType:
		t := FischerReferenceType
		out.ReferenceType = &t
		out.Reference = in.FischerReference
	}

	return nil
}
