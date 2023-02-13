/*
Copyright 2021-2022 Gravitational, Inc.

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

package main

import (
	"fmt"
	"strings"

	"github.com/gravitational/teleport/schemagen"
	"golang.org/x/exp/slices"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crdtools "sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"
)

const k8sKindPrefix = "Teleport"

// Add names to this array when adding support to new Teleport resources that could conflict with Kubernetes
var kubernetesReservedNames = []string{"role"}

func CustomResourceDefinition(root *schemagen.RootSchema, groupName string) apiextv1.CustomResourceDefinition {
	crd := apiextv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", strings.ToLower(k8sKindPrefix+root.PluralName), groupName),
		},
		Spec: apiextv1.CustomResourceDefinitionSpec{
			Group: groupName,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:       k8sKindPrefix + root.Kind,
				ListKind:   k8sKindPrefix + root.Kind + "List",
				Plural:     strings.ToLower(k8sKindPrefix + root.PluralName),
				Singular:   strings.ToLower(k8sKindPrefix + root.Name),
				ShortNames: getShortNames(root),
			},
			Scope: apiextv1.NamespaceScoped,
		},
	}

	// This part parses the types not coming from the protobuf (the status)
	// We instantiate a parser, load the relevant packages in it and look for
	// the package we need. The package is then loaded to the parser, a schema is
	// generated and used in the CRD

	registry := &markers.Registry{}
	// CRD markers contain special markers used by the parser to discover properties
	// e.g. `+kubebuilder:validation:Minimum=0`
	crdmarkers.Register(registry)
	parser := &crdtools.Parser{
		Collector: &markers.Collector{Registry: registry},
		Checker:   &loader.TypeChecker{},
	}

	// Some types are special and require manual overrides, like metav1.Time.
	crdtools.AddKnownTypes(parser)

	// hack, we should be able to retrieve the path instead
	pkgs, err := loader.LoadRoots("../...")
	if err != nil {
		fmt.Printf("parser error: %s", err)
	}

	for i, schemaVersion := range root.Versions {

		var statusType crdtools.TypeIdent
		versionName := schemaVersion.Version
		schema := schemaVersion.Schema
		for _, pkg := range pkgs {
			// This if is a bit janky, condition checking should be stronger
			if pkg.Name == versionName {
				parser.NeedPackage(pkg)
				statusType = crdtools.TypeIdent{
					Package: pkg,
					Name:    fmt.Sprintf("%s%sStatus", k8sKindPrefix, root.Kind),
				}
				// Kubernetes CRDs don't support $ref in openapi schemas, we need a flattened schema
				parser.NeedFlattenedSchemaFor(statusType)
			}
		}

		crd.Spec.Versions = append(crd.Spec.Versions, apiextv1.CustomResourceDefinitionVersion{
			Name:   versionName,
			Served: true,
			// Storage the first version available.
			Storage: i == 0,
			Subresources: &apiextv1.CustomResourceSubresources{
				Status: &apiextv1.CustomResourceSubresourceStatus{},
			},
			Schema: &apiextv1.CustomResourceValidation{
				OpenAPIV3Schema: &apiextv1.JSONSchemaProps{
					Type:        "object",
					Description: fmt.Sprintf("%s is the Schema for the %s API", root.Kind, root.PluralName),
					Properties: map[string]apiextv1.JSONSchemaProps{
						"apiVersion": {
							Type:        "string",
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources",
						},
						"kind": {
							Type:        "string",
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds",
						},
						"metadata": {Type: "object"},
						"spec":     schema.JSONSchemaProps,
						"status":   parser.FlattenedSchemata[statusType],
					},
				},
			},
		})
	}
	return crd
}

// getShortNames returns the schema short names while ensuring they won't conflict with existing Kubernetes resources
// See https://github.com/gravitational/teleport/issues/17587 and https://github.com/kubernetes/kubernetes/issues/113227
func getShortNames(root *schemagen.RootSchema) []string {
	if slices.Contains(kubernetesReservedNames, root.Name) {
		return []string{}
	}
	return []string{root.Name, root.PluralName}
}
