// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/gravitational/trace"
	yaml "gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/utils"
)

// kindVersionObject is used for unmarshaling Teleport resources so we can branch on
// their kinds and/or versions.
type kindVersionObject struct {
	Kind    string
	Version string
}

// jsonConverter is a function that transforms JSON bytes into a Teleport
// Terraform resource. The input is JSON because the script converts resource
// representations (e.g., gogo-proto and RFD 153) into JSON before converting.
type jsonConverter func(data []byte) (tfgen.Resource, error)

// kubeConversionAttributes configures the way the converter transforms a tctl
// resource into a Kubernetes resource.
type kubeConversionAttributes struct {
	// subKindToResourceKind is a map of sub_kind values that, if present on
	// a tctl resource, determine the kind of a matching Kubernetes CRD.
	subKindToResourceKind map[string]string
	// apiVersion specifies the apiVersion value in a Kubernetes resource.
	apiVersion string
	// kind specifies the value of kind in a Kubernetes resource.
	kind string
	// ignoredSpecFields are fields in a tctl resource spec that the
	// converter removes from the final Kubernetes resource.
	ignoredSpecFields []string
	// requiredVersion is an optional version to require tctl resources to
	// have for conversion. We use this for CRDs that are pinned to a
	// specific resource version.
	requiredVersion string
	// scopeNotInCRD indicates whether a top-level scope field is absent in
	// the Kubernetes resource while present in the tctl equivalent.
	scopeNotInCRD bool
}

// conversionRule is a configuration for how to transform a given kind of tctl
// resource into a Terraform provider and Kubernetes operator resource.
type conversionRule struct {
	// toTerraformResource converts JSON bytes (returned by normalizing resource
	// YAML) into a Teleport Terraform resource.
	toTerraformResource jsonConverter
	// kubernetes is a set of configuration options for converting a tctl
	// resource into a Kubernetes resource.
	kubernetes kubeConversionAttributes
	// terraformResourceType is the type of the Terraform resource if it
	// differs from the value of kind in the corresponding tctl resource.
	terraformResourceType string
	// generateOpts are applied when converting to HCL.
	generateOpts []tfgen.GenerateOpt
}

// unsupportedResource is an error indicating that a tctl resource does not have
// a corresponding resource for a particular infrastructure as code tool. Errors
// for unsupported resources must use this type for consistency.
type unsupportedResource struct {
	kind    string // Resource kind, e.g., "role".
	version string // Optional resource version for printing.
	tool    string // Infrastructure as code tool. Used for error messages.
}

// Error prints the error message for unsupportedResource.
func (r unsupportedResource) Error() string {
	var verSuffix string
	if r.version != "" {
		verSuffix = " (" + r.version + ")"
	}
	return fmt.Sprintf(`%v does not support resource kind %v%v`, r.tool, r.kind, verSuffix)
}

// resourceConfig maps the kind values of resources supported by the Terraform
// provider to functions for converting JSON to HCL, as well as to functions for
// converting Teleport resource types to Kubernetes resources. There are three
// patterns for applying the conversion:
//  1. For legacy gogo-proto types, the YAML/JSON type directly maps to the
//     Protobuf-generated type, which includes json struct tags, so we can
//     unmarshal directly using utils.FastUnmarshal.
//  2. For types based on non-gogo Protobuf messages, unmarshal using
//     protojson.Unmarshal.
//  3. For resources that include a header, call utils.FastUnmarshal into the
//     internal representation of the type, then convert to a Protobuf-based type
//     and wrap with a header.
var resourceConfig = map[string]conversionRule{
	"role": {
		toTerraformResource: func(data []byte) (tfgen.Resource, error) {
			var r types.RoleV6
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid Teleport role: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion:      "resources.teleport.dev/v1",
			kind:            "TeleportRoleV8",
			requiredVersion: "v8",
		},
	},
	"user": {
		toTerraformResource: func(data []byte) (tfgen.Resource, error) {
			var r types.UserV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid user: %w", err)
			}
			return &r, nil
		},
		generateOpts: []tfgen.GenerateOpt{
			tfgen.WithOmitField("spec.status"),
			tfgen.WithOmitField("spec.created_by"),
			tfgen.WithOmitField("spec.expires"),
		},
		kubernetes: kubeConversionAttributes{
			apiVersion:        "resources.teleport.dev/v2",
			kind:              "TeleportUser",
			ignoredSpecFields: []string{"local_auth", "expires", "created_by", "status"},
		},
	},
	"token": {
		toTerraformResource: func(data []byte) (tfgen.Resource, error) {
			var r types.ProvisionTokenV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid token: %w", err)
			}
			return &r, nil
		},
		kubernetes: kubeConversionAttributes{
			apiVersion: "resources.teleport.dev/v2",
			kind:       "TeleportProvisionToken",
		},
		terraformResourceType: "teleport_provision_token",
	},
	"cluster_auth_preference": {
		toTerraformResource: func(data []byte) (tfgen.Resource, error) {
			var r types.AuthPreferenceV2
			if err := utils.FastUnmarshal(data, &r); err != nil {
				return nil, trace.Errorf("invalid cluster_auth_preference: %w", err)
			}
			return &r, nil
		},
		terraformResourceType: "teleport_auth_preference",
	},
}

// fieldPathsWithPrefix recursively traverses m and outputs a slice of field
// paths separated by dots. All returned field paths begin with prefix.
func fieldPathsWithPrefix(m map[string]any, prefix string) []string {
	out := []string{}
	for k, v := range m {
		newPrefix := k
		if prefix != "" {
			newPrefix = prefix + "." + k
		}
		out = append(out, newPrefix)
		if a, ok := v.(map[string]any); ok {
			out = append(out, fieldPathsWithPrefix(a, newPrefix)...)
		}
	}
	return out
}

// fieldPaths traverses m and outputs a slice of dot-separated field paths
func fieldPaths(m map[string]any) []string {
	return fieldPathsWithPrefix(m, "")
}

const hclComment = "#"

// convertYAMLToHCL takes a single tctl resource YAML document, converts it to
// an HCL resource configuration, writing out the resulting HCL object to w.
func convertYAMLToHCL(w io.Writer, r io.Reader) error {
	yamlBytes, err := io.ReadAll(r)
	if err != nil {
		return trace.Errorf("unable to read input YAML: %w", err)
	}

	jsonbytes, err := utils.ToJSON(yamlBytes)
	if err != nil {
		return trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)
	}

	var o kindVersionObject
	if err = json.Unmarshal(jsonbytes, &o); err != nil {
		return trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	convert, ok := resourceConfig[o.Kind]
	if !ok {
		return unsupportedResource{
			kind: o.Kind,
			tool: "Terraform",
		}
	}

	res, err := convert.toTerraformResource(jsonbytes)
	if err != nil {
		return err
	}

	var m map[string]any
	if err = json.Unmarshal(jsonbytes, &m); err != nil {
		return trace.Errorf("unable to read JSON from the input resource: %w", err)
	}

	var opts []tfgen.GenerateOpt
	if convert.terraformResourceType != "" {
		opts = append(opts, tfgen.WithResourceType(convert.terraformResourceType))
	}

	if convert.generateOpts != nil {
		opts = append(opts, convert.generateOpts...)
	}

	// We need to preserve fields listed in the original resource that have
	// zero values. tfgen.Generate omits zeroed values by default. The
	// tfgen.GenerateOpt WithFieldComment is the only one that instructs the
	// generator to preserve a zeroed field. We retrieve a slice of field
	// paths by traversing all fields in the original JSON resource object.
	// Then we pass each field path to a WithFieldComment option to preserve
	// all fields from the original resource.
	for _, p := range fieldPaths(m) {
		opts = append(opts, tfgen.WithFieldComment(p, ""))
	}

	outbytes, err := tfgen.Generate(res, opts...)
	if err != nil {
		return trace.Errorf("unable to convert the provided YAML manifest into HCL: %w", err)
	}

	// At this point, fields in the output that we passed to
	// WithFieldComment include an empty HCL comment marker.  Write out the
	// result line by line, ignoring the empty comment markers.
	scanner := bufio.NewScanner(strings.NewReader(string(outbytes)))
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != hclComment {
			w.Write(scanner.Bytes())
			w.Write([]byte("\n"))
		}
	}

	return nil
}

// sepPattern represents a YAML document separator. Used for splitting YAML
// documents to convert individual resources.
var sepPattern = regexp.MustCompile(`(?m)^---\s*$`)

// convertAllYAMLToHCL takes one or more tctl resource YAML documents with
// possible document separators in r, converts them to HCL resource
// configurations in a single document, writing out the document to w.
func convertAllYAMLToHCL(w io.Writer, r io.Reader) error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return trace.Errorf("could not read input document: %w", err)
	}

	docs := sepPattern.Split(buf.String(), -1)
	for i, doc := range docs {
		if doc == "" {
			// Skip empty documents, e.g., because of leading separator
			continue
		}
		if err := convertYAMLToHCL(w, strings.NewReader(doc)); err != nil {
			return err
		}

		if i+1 == len(docs) {
			break
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return trace.Errorf("unable to write out a document separator: %w", err)
		}
	}

	return nil
}

// convertAllYAMLToKubernetes takes one or more tctl resource YAML documents
// with possible document separators in r, converts them to Kubernetes resource
// manifests in a single document, writing out the document to w.
func convertAllYAMLToKubernetes(w io.Writer, r io.Reader) error {
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		return trace.Errorf("could not read input document: %w", err)
	}

	docs := sepPattern.Split(buf.String(), -1)
	for i, doc := range docs {
		if doc == "" {
			// Skip empty documents, e.g., because of leading separator
			continue
		}
		if err := convertYAMLToKubernetes(w, strings.NewReader(doc)); err != nil {
			return err
		}

		if i+1 == len(docs) {
			break
		}
		if _, err := w.Write([]byte("---\n")); err != nil {
			return trace.Errorf("unable to write out a document separator: %w", err)
		}
	}

	return nil
}

// convertYAMLToKubernetes takes a single tctl resource YAML document and
// converts it to a Kubernetes manifest, writing out the resulting HCL to w.
func convertYAMLToKubernetes(w io.Writer, r io.Reader) error {
	yamlBytes, err := io.ReadAll(r)
	if err != nil {
		return trace.Errorf("unable to read input YAML: %w", err)
	}

	jsonbytes, err := utils.ToJSON(yamlBytes)
	if err != nil {
		return trace.Errorf("unable to process input YAML as JSON (which we need to do to convert it to a Teleport resource type): %w", err)
	}

	var original map[string]any
	if err = json.Unmarshal(jsonbytes, &original); err != nil {
		return trace.Errorf("unable to convert the input resource to a mapping: %w", err)
	}

	var o kindVersionObject
	if err = yaml.Unmarshal(jsonbytes, &o); err != nil {
		return trace.Errorf("unable to detect a kind in the input resource: %w", err)
	}

	convert, ok := resourceConfig[o.Kind]
	if !ok || (convert.kubernetes.apiVersion == "" && convert.kubernetes.kind == "") {
		return unsupportedResource{
			kind: o.Kind,
			tool: "Kubernetes",
		}
	}

	// If there's a version requirement, reject the resource as unsupported.
	if convert.kubernetes.requiredVersion != "" && o.Version != convert.kubernetes.requiredVersion {
		return unsupportedResource{
			kind:    o.Kind,
			tool:    "Kubernetes",
			version: o.Version,
		}
	}

	if _, hasScope := original["scope"]; convert.kubernetes.scopeNotInCRD && hasScope {
		return trace.Errorf("converting tctl resources to Kubernetes does not yet support scoped %v resources", o.Kind)
	}

	// Kubernetes resources have the same structure as tctl resources with a
	// few exceptions. To convert a tctl resource to a Kubernetes resource,
	// we add an apiVersion and kind suitable for Kubernetes, remove the
	// version (which is encoded in the kind), and handle two edge cases:
	//  - Some have a sub-kind that determines the CRD kind
	//  - Some have fields that the Teleport Kubernetes operator ignores

	original["kind"] = convert.kubernetes.kind
	original["apiVersion"] = convert.kubernetes.apiVersion
	delete(original, "version")

	if convert.kubernetes.subKindToResourceKind != nil {
		sk, ok := original["sub_kind"]
		if !ok || sk == "" {
			return trace.Errorf("resource %v needs a sub_kind", o.Kind)
		}
		skval, ok := convert.kubernetes.subKindToResourceKind[sk.(string)]
		if !ok {
			return trace.Errorf("unrecognized sub_kind in resource %v", o.Kind)
		}
		original["kind"] = skval
	}
	delete(original, "sub_kind")

	if specmap, ok := original["spec"].(map[string]any); ok {
		for _, f := range convert.kubernetes.ignoredSpecFields {
			delete(specmap, f)
		}
	}

	// Kubernetes resources move the metadata.description in the tctl
	// resource to metadata.annotations.description
	if meta, ok := original["metadata"].(map[string]any); ok {
		if desc, ok := meta["description"]; ok {
			annotations, _ := meta["annotations"].(map[string]any)
			if annotations == nil {
				annotations = make(map[string]any)
			}
			annotations["description"] = desc
			meta["annotations"] = annotations
			delete(meta, "description")
		}
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(&original); err != nil {
		return trace.Errorf("unable to convert %v to Kubernetes YAML: %w", o.Kind, err)
	}

	return nil
}

func main() {
	format := flag.String("format", "", "hcl or kube")
	flag.Parse()

	var err error
	switch *format {
	case "kube":
		err = convertAllYAMLToKubernetes(os.Stdout, os.Stdin)

	case "hcl":
		err = convertAllYAMLToHCL(os.Stdout, os.Stdin)
	default:
		fmt.Fprintf(os.Stderr, `The format flag must be hcl or kube. Got: %v`, *format)
		os.Exit(1)
	}

	if err == nil {
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "Cannot convert resource(s): %v", err)
	if errors.As(err, &unsupportedResource{}) {
		// We reserve 2 for unsupported resources so the invoking shell
		// knows that execution otherwise took place as expected.
		os.Exit(2)
	}

	os.Exit(1)
}
