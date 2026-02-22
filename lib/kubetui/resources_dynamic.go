/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package kubetui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

// discoveryDoneMsg is sent when API resource discovery completes.
type discoveryDoneMsg struct {
	added int
	err   error
}

// discoverAndRegisterResources discovers all API resources from the server
// and registers dynamic ResourceTypes for any not already known.
func discoverAndRegisterResources(client *Client) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		lists, err := client.DiscoverAllResources(ctx)
		// Discovery returns partial results on error, so proceed even if err != nil.

		added := 0
		for _, list := range lists {
			gv, parseErr := schema.ParseGroupVersion(list.GroupVersion)
			if parseErr != nil {
				continue
			}
			for _, res := range list.APIResources {
				// Skip subresources (e.g. pods/log, pods/exec).
				if strings.Contains(res.Name, "/") {
					continue
				}
				// Skip resources that don't support list.
				if !hasVerb(res.Verbs, "list") {
					continue
				}
				// Skip resources already registered by typed definitions.
				cmd := strings.ToLower(res.Name)
				if _, exists := resourceTypeByCommand[cmd]; exists {
					continue
				}

				gvr := schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: res.Name,
				}
				rt := makeDynamicResourceType(res, gvr)
				AllResourceTypes = append(AllResourceTypes, rt)
				resourceTypeByCommand[rt.Command] = rt
				for _, alias := range rt.Aliases {
					resourceTypeByCommand[alias] = rt
				}
				added++
			}
		}

		return discoveryDoneMsg{added: added, err: err}
	}
}

func hasVerb(verbs metav1.Verbs, verb string) bool {
	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}

// makeDynamicResourceType builds a ResourceType backed by the dynamic client.
func makeDynamicResourceType(res metav1.APIResource, gvr schema.GroupVersionResource) *ResourceType {
	namespaced := res.Namespaced
	cmd := strings.ToLower(res.Name)

	aliases := make([]string, 0, len(res.ShortNames)+1)
	if res.SingularName != "" && res.SingularName != cmd {
		aliases = append(aliases, res.SingularName)
	}
	aliases = append(aliases, res.ShortNames...)

	var columns []Column
	if namespaced {
		columns = []Column{
			{Header: "NAMESPACE", Width: 20},
			{Header: "NAME", Flexible: true},
			{Header: "AGE", Width: 10},
		}
	} else {
		columns = []Column{
			{Header: "NAME", Flexible: true},
			{Header: "AGE", Width: 10},
		}
	}

	rt := &ResourceType{
		Name:       res.Kind,
		Command:    cmd,
		Aliases:    aliases,
		Namespaced: namespaced,
		Columns:    columns,
		Actions:    []string{"describe", "edit", "delete"},
	}

	// Capture gvr and namespaced in closures.
	rt.FetchFunc = func(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
		ns := namespace
		if !namespaced {
			ns = ""
		}
		list, fetchErr := client.DynamicList(ctx, gvr, ns)
		if fetchErr != nil {
			return nil, fetchErr
		}
		resources := make([]Resource, 0, len(list.Items))
		for _, item := range list.Items {
			r := Resource{
				Name: item.GetName(),
			}
			if namespaced {
				r.Namespace = item.GetNamespace()
				r.Cells = []string{formatAge(item.GetCreationTimestamp().Time)}
			} else {
				r.Cells = []string{formatAge(item.GetCreationTimestamp().Time)}
			}
			resources = append(resources, r)
		}
		return resources, nil
	}

	rt.DescribeFunc = func(ctx context.Context, client *Client, namespace, name string) (string, error) {
		ns := namespace
		if !namespaced {
			ns = ""
		}
		obj, getErr := client.DynamicGet(ctx, gvr, ns, name)
		if getErr != nil {
			return "", getErr
		}
		// Strip managedFields for cleaner output.
		unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
		data, marshalErr := yaml.Marshal(obj.Object)
		if marshalErr != nil {
			return "", marshalErr
		}
		return string(data), nil
	}

	rt.GetYAMLFunc = func(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
		ns := namespace
		if !namespaced {
			ns = ""
		}
		obj, getErr := client.DynamicGet(ctx, gvr, ns, name)
		if getErr != nil {
			return nil, getErr
		}
		unstructured.RemoveNestedField(obj.Object, "metadata", "managedFields")
		return yaml.Marshal(obj.Object)
	}

	rt.UpdateFunc = func(ctx context.Context, client *Client, namespace, _ string, yamlData []byte) error {
		ns := namespace
		if !namespaced {
			ns = ""
		}
		var obj unstructured.Unstructured
		parsed := make(map[string]interface{})
		if unmarshalErr := yaml.Unmarshal(yamlData, &parsed); unmarshalErr != nil {
			return fmt.Errorf("invalid YAML: %w", unmarshalErr)
		}
		obj.Object = parsed
		return client.DynamicUpdate(ctx, gvr, ns, &obj)
	}

	rt.DeleteFunc = func(ctx context.Context, client *Client, namespace, name string) error {
		ns := namespace
		if !namespaced {
			ns = ""
		}
		return client.DynamicDelete(ctx, gvr, ns, name)
	}

	return rt
}
