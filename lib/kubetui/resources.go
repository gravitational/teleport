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
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// ResourceType defines a Kubernetes resource kind for the TUI.
type ResourceType struct {
	Name         string
	Command      string
	Aliases      []string
	Namespaced   bool
	Columns      []Column
	FetchFunc    func(ctx context.Context, client *Client, namespace string) ([]Resource, error)
	Actions      []string
	DescribeFunc func(ctx context.Context, client *Client, namespace, name string) (string, error)
	ContentFunc  func(ctx context.Context, client *Client, namespace, name string) (string, error)
	RowStyleFunc func(resource Resource) lipgloss.Style
	GetYAMLFunc  func(ctx context.Context, client *Client, namespace, name string) ([]byte, error)
	UpdateFunc   func(ctx context.Context, client *Client, namespace, name string, yamlData []byte) error
	DeleteFunc   func(ctx context.Context, client *Client, namespace, name string) error
}

// Column describes a single column in the resource table.
type Column struct {
	Header   string
	Width    int
	Flexible bool
}

// Resource is a generic row in the resource table.
type Resource struct {
	Namespace string
	Name      string
	Cells     []string // values for columns after NAMESPACE and NAME
}

// AllResourceTypes lists every supported resource type.
// Tier-1 typed resources are appended from resources_tier1.go init().
// Dynamic resources are appended at runtime via discovery.
var AllResourceTypes = []*ResourceType{
	ResourcePods,
	ResourceDeployments,
	ResourceServices,
	ResourceConfigMaps,
	ResourceSecrets,
	ResourceStatefulSets,
	ResourceDaemonSets,
	ResourceReplicaSets,
	ResourceJobs,
	ResourceCronJobs,
	ResourceNodes,
	ResourcePersistentVolumes,
	ResourcePersistentVolumeClaims,
	ResourceServiceAccounts,
	ResourceIngresses,
	ResourceHPAs,
}

var resourceTypeByCommand map[string]*ResourceType

func init() {
	resourceTypeByCommand = make(map[string]*ResourceType)
	for _, rt := range AllResourceTypes {
		resourceTypeByCommand[rt.Command] = rt
		for _, alias := range rt.Aliases {
			resourceTypeByCommand[alias] = rt
		}
	}
}

// LookupResourceType returns the ResourceType matching cmd, or nil.
func LookupResourceType(cmd string) *ResourceType {
	return resourceTypeByCommand[cmd]
}

// ---------------------------------------------------------------------------
// Pods
// ---------------------------------------------------------------------------

var ResourcePods = &ResourceType{
	Name:       "Pods",
	Command:    "pods",
	Aliases:    []string{"pod", "po"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "READY", Width: 7},
		{Header: "STATUS", Width: 12},
		{Header: "RESTARTS", Width: 10},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchPods,
	Actions:      []string{"logs", "describe", "port-forward", "copy", "edit", "delete", "exec"},
	DescribeFunc: describePod,
	GetYAMLFunc:  getPodYAML,
	UpdateFunc:   updatePod,
	DeleteFunc:   deletePod,
	RowStyleFunc: func(r Resource) lipgloss.Style {
		if len(r.Cells) >= 2 {
			return podStatusStyle(r.Cells[1]) // STATUS cell
		}
		return lipgloss.NewStyle()
	},
}

func fetchPods(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	pods, err := client.ListPods(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(pods))
	for _, pod := range pods {
		resources = append(resources, Resource{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Cells: []string{
				podReadyCount(pod),
				string(pod.Status.Phase),
				fmt.Sprintf("%d", podRestartCount(pod)),
				formatAge(pod.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describePod(ctx context.Context, client *Client, namespace, name string) (string, error) {
	pod, err := client.DescribePod(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderPodDescribe(pod), nil
}

func renderPodDescribe(pod *v1.Pod) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:        ") + pod.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:   ") + pod.Namespace + "\n")
	b.WriteString(labelStyle.Render("Node:        ") + pod.Spec.NodeName + "\n")
	b.WriteString(labelStyle.Render("Status:      ") + string(pod.Status.Phase) + "\n")
	b.WriteString(labelStyle.Render("IP:          ") + pod.Status.PodIP + "\n")
	if pod.Status.StartTime != nil {
		b.WriteString(labelStyle.Render("Start Time:  ") + pod.Status.StartTime.String() + "\n")
	}
	b.WriteString("\n")

	if len(pod.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels") + "\n")
		for k, v := range pod.Labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	b.WriteString(sectionStyle.Render("Containers") + "\n")
	for _, c := range pod.Spec.Containers {
		b.WriteString(labelStyle.Render("  "+c.Name) + "\n")
		b.WriteString(fmt.Sprintf("    Image:   %s\n", c.Image))
		if len(c.Ports) > 0 {
			ports := make([]string, 0, len(c.Ports))
			for _, p := range c.Ports {
				ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
			}
			b.WriteString(fmt.Sprintf("    Ports:   %s\n", strings.Join(ports, ", ")))
		}

		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name == c.Name {
				b.WriteString(fmt.Sprintf("    Ready:   %t\n", cs.Ready))
				b.WriteString(fmt.Sprintf("    Restarts: %d\n", cs.RestartCount))
				if cs.State.Running != nil {
					b.WriteString(fmt.Sprintf("    State:   Running (since %s)\n", cs.State.Running.StartedAt.String()))
				} else if cs.State.Waiting != nil {
					b.WriteString(fmt.Sprintf("    State:   Waiting (%s)\n", cs.State.Waiting.Reason))
				} else if cs.State.Terminated != nil {
					b.WriteString(fmt.Sprintf("    State:   Terminated (%s)\n", cs.State.Terminated.Reason))
				}
			}
		}
		b.WriteString("\n")
	}

	if len(pod.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range pod.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s\n", c.Type, string(c.Status)))
		}
		b.WriteString("\n")
	}

	if len(pod.Spec.Volumes) > 0 {
		b.WriteString(sectionStyle.Render("Volumes") + "\n")
		for _, v := range pod.Spec.Volumes {
			b.WriteString(fmt.Sprintf("  %s\n", v.Name))
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Services
// ---------------------------------------------------------------------------

var ResourceServices = &ResourceType{
	Name:       "Services",
	Command:    "svc",
	Aliases:    []string{"services", "service"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "TYPE", Width: 12},
		{Header: "CLUSTER-IP", Width: 16},
		{Header: "PORTS", Width: 24},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchServices,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeService,
	GetYAMLFunc:  getServiceYAML,
	UpdateFunc:   updateService,
	DeleteFunc:   deleteService,
}

func fetchServices(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	services, err := client.ListServices(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(services))
	for _, svc := range services {
		resources = append(resources, Resource{
			Namespace: svc.Namespace,
			Name:      svc.Name,
			Cells: []string{
				string(svc.Spec.Type),
				svc.Spec.ClusterIP,
				formatServicePorts(svc.Spec.Ports),
				formatAge(svc.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeService(ctx context.Context, client *Client, namespace, name string) (string, error) {
	svc, err := client.GetService(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderServiceDescribe(svc), nil
}

func renderServiceDescribe(svc *v1.Service) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + svc.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + svc.Namespace + "\n")
	b.WriteString(labelStyle.Render("Type:            ") + string(svc.Spec.Type) + "\n")
	b.WriteString(labelStyle.Render("Cluster IP:      ") + svc.Spec.ClusterIP + "\n")
	if svc.Spec.ExternalName != "" {
		b.WriteString(labelStyle.Render("External Name:   ") + svc.Spec.ExternalName + "\n")
	}
	if len(svc.Spec.ExternalIPs) > 0 {
		b.WriteString(labelStyle.Render("External IPs:    ") + strings.Join(svc.Spec.ExternalIPs, ", ") + "\n")
	}
	b.WriteString(labelStyle.Render("Session Affinity:") + " " + string(svc.Spec.SessionAffinity) + "\n")
	b.WriteString("\n")

	if len(svc.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels") + "\n")
		for k, v := range svc.Labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	if len(svc.Spec.Selector) > 0 {
		b.WriteString(sectionStyle.Render("Selector") + "\n")
		for k, v := range svc.Spec.Selector {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	if len(svc.Spec.Ports) > 0 {
		b.WriteString(sectionStyle.Render("Ports") + "\n")
		for _, p := range svc.Spec.Ports {
			line := fmt.Sprintf("  %s %d/%s", p.Name, p.Port, p.Protocol)
			if p.NodePort != 0 {
				line += fmt.Sprintf(" NodePort:%d", p.NodePort)
			}
			if p.TargetPort.String() != "" {
				line += fmt.Sprintf(" -> %s", p.TargetPort.String())
			}
			b.WriteString(line + "\n")
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// ConfigMaps
// ---------------------------------------------------------------------------

var ResourceConfigMaps = &ResourceType{
	Name:       "ConfigMaps",
	Command:    "cm",
	Aliases:    []string{"configmaps", "configmap"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "DATA", Width: 6},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchConfigMaps,
	Actions:      []string{"view", "describe", "edit", "delete"},
	DescribeFunc: describeConfigMap,
	ContentFunc:  viewConfigMapContents,
	GetYAMLFunc:  getConfigMapYAML,
	UpdateFunc:   updateConfigMap,
	DeleteFunc:   deleteConfigMap,
}

func fetchConfigMaps(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	cms, err := client.ListConfigMaps(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(cms))
	for _, cm := range cms {
		resources = append(resources, Resource{
			Namespace: cm.Namespace,
			Name:      cm.Name,
			Cells: []string{
				fmt.Sprintf("%d", len(cm.Data)+len(cm.BinaryData)),
				formatAge(cm.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeConfigMap(ctx context.Context, client *Client, namespace, name string) (string, error) {
	cm, err := client.GetConfigMap(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderConfigMapDescribe(cm), nil
}

func renderConfigMapDescribe(cm *v1.ConfigMap) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:        ") + cm.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:   ") + cm.Namespace + "\n")
	b.WriteString("\n")

	if len(cm.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels") + "\n")
		for k, v := range cm.Labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	if len(cm.Data) > 0 {
		b.WriteString(sectionStyle.Render("Data") + "\n")
		for k, v := range cm.Data {
			b.WriteString(fmt.Sprintf("  %s (%d bytes)\n", k, len(v)))
		}
		b.WriteString("\n")
	}

	if len(cm.BinaryData) > 0 {
		b.WriteString(sectionStyle.Render("Binary Data") + "\n")
		for k, v := range cm.BinaryData {
			b.WriteString(fmt.Sprintf("  %s (%d bytes)\n", k, len(v)))
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Secrets
// ---------------------------------------------------------------------------

var ResourceSecrets = &ResourceType{
	Name:       "Secrets",
	Command:    "secrets",
	Aliases:    []string{"secret", "sec"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "TYPE", Width: 30},
		{Header: "DATA", Width: 6},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchSecrets,
	Actions:      []string{"view", "describe", "edit", "delete"},
	DescribeFunc: describeSecret,
	ContentFunc:  viewSecretContents,
	GetYAMLFunc:  getSecretYAML,
	UpdateFunc:   updateSecret,
	DeleteFunc:   deleteSecret,
}

func fetchSecrets(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	secrets, err := client.ListSecrets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(secrets))
	for _, s := range secrets {
		resources = append(resources, Resource{
			Namespace: s.Namespace,
			Name:      s.Name,
			Cells: []string{
				string(s.Type),
				fmt.Sprintf("%d", len(s.Data)),
				formatAge(s.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeSecret(ctx context.Context, client *Client, namespace, name string) (string, error) {
	secret, err := client.GetSecret(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderSecretDescribe(secret), nil
}

func renderSecretDescribe(secret *v1.Secret) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:        ") + secret.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:   ") + secret.Namespace + "\n")
	b.WriteString(labelStyle.Render("Type:        ") + string(secret.Type) + "\n")
	b.WriteString("\n")

	if len(secret.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels") + "\n")
		for k, v := range secret.Labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	if len(secret.Data) > 0 {
		b.WriteString(sectionStyle.Render("Data") + "\n")
		for k, v := range secret.Data {
			b.WriteString(fmt.Sprintf("  %s (%d bytes)\n", k, len(v)))
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// Content view functions for ConfigMaps and Secrets
// ---------------------------------------------------------------------------

func viewConfigMapContents(ctx context.Context, client *Client, namespace, name string) (string, error) {
	cm, err := client.GetConfigMap(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderConfigMapContents(cm), nil
}

func renderConfigMapContents(cm *v1.ConfigMap) string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)

	var b strings.Builder

	// Sort keys for stable output.
	dataKeys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		dataKeys = append(dataKeys, k)
	}
	sort.Strings(dataKeys)

	binaryKeys := make([]string, 0, len(cm.BinaryData))
	for k := range cm.BinaryData {
		binaryKeys = append(binaryKeys, k)
	}
	sort.Strings(binaryKeys)

	if len(dataKeys) == 0 && len(binaryKeys) == 0 {
		b.WriteString("(no data)\n")
		return b.String()
	}

	for i, k := range dataKeys {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionStyle.Render(k) + "\n")
		b.WriteString(cm.Data[k])
		if !strings.HasSuffix(cm.Data[k], "\n") {
			b.WriteString("\n")
		}
	}

	for _, k := range binaryKeys {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionStyle.Render(k) + "\n")
		b.WriteString(keyStyle.Render(fmt.Sprintf("(binary, %d bytes)", len(cm.BinaryData[k]))) + "\n")
	}

	return b.String()
}

func viewSecretContents(ctx context.Context, client *Client, namespace, name string) (string, error) {
	secret, err := client.GetSecret(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderSecretContents(secret), nil
}

func renderSecretContents(secret *v1.Secret) string {
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)

	var b strings.Builder

	// Sort keys for stable output.
	keys := make([]string, 0, len(secret.Data))
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		b.WriteString("(no data)\n")
		return b.String()
	}

	for i, k := range keys {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(sectionStyle.Render(k) + "\n")
		val := secret.Data[k]
		if isPrintable(val) {
			b.WriteString(string(val))
			if len(val) > 0 && val[len(val)-1] != '\n' {
				b.WriteString("\n")
			}
		} else {
			b.WriteString(keyStyle.Render(fmt.Sprintf("(binary, %d bytes)", len(val))) + "\n")
		}
	}

	return b.String()
}

// isPrintable reports whether data is valid UTF-8 consisting entirely of
// printable runes (graphic characters, spaces, tabs, and newlines).
func isPrintable(data []byte) bool {
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 {
			return false
		}
		if !unicode.IsPrint(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
		data = data[size:]
	}
	return true
}

// ---------------------------------------------------------------------------
// Deployments
// ---------------------------------------------------------------------------

var ResourceDeployments = &ResourceType{
	Name:       "Deployments",
	Command:    "deploy",
	Aliases:    []string{"deployments", "deployment", "dep"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "READY", Width: 9},
		{Header: "UP-TO-DATE", Width: 12},
		{Header: "AVAILABLE", Width: 11},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchDeployments,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeDeployment,
	GetYAMLFunc:  getDeploymentYAML,
	UpdateFunc:   updateDeployment,
	DeleteFunc:   deleteDeployment,
}

func fetchDeployments(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	deployments, err := client.ListDeployments(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(deployments))
	for _, d := range deployments {
		desired := int32(1)
		if d.Spec.Replicas != nil {
			desired = *d.Spec.Replicas
		}
		ready := d.Status.ReadyReplicas
		resources = append(resources, Resource{
			Namespace: d.Namespace,
			Name:      d.Name,
			Cells: []string{
				fmt.Sprintf("%d/%d", ready, desired),
				fmt.Sprintf("%d", d.Status.UpdatedReplicas),
				fmt.Sprintf("%d", d.Status.AvailableReplicas),
				formatAge(d.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeDeployment(ctx context.Context, client *Client, namespace, name string) (string, error) {
	dep, err := client.GetDeployment(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderDeploymentDescribe(dep), nil
}

func renderDeploymentDescribe(dep *appsv1.Deployment) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + dep.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + dep.Namespace + "\n")
	desired := int32(1)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	b.WriteString(labelStyle.Render("Replicas:        ") + fmt.Sprintf("%d desired | %d updated | %d total | %d available | %d unavailable",
		desired,
		dep.Status.UpdatedReplicas,
		dep.Status.Replicas,
		dep.Status.AvailableReplicas,
		dep.Status.UnavailableReplicas,
	) + "\n")
	b.WriteString(labelStyle.Render("Strategy:        ") + string(dep.Spec.Strategy.Type) + "\n")
	b.WriteString("\n")

	if len(dep.Labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels") + "\n")
		for k, v := range dep.Labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	if dep.Spec.Template.Spec.Containers != nil {
		b.WriteString(sectionStyle.Render("Pod Template") + "\n")
		for _, c := range dep.Spec.Template.Spec.Containers {
			b.WriteString(labelStyle.Render("  "+c.Name) + "\n")
			b.WriteString(fmt.Sprintf("    Image:   %s\n", c.Image))
			if len(c.Ports) > 0 {
				ports := make([]string, 0, len(c.Ports))
				for _, p := range c.Ports {
					ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
				}
				b.WriteString(fmt.Sprintf("    Ports:   %s\n", strings.Join(ports, ", ")))
			}
		}
		b.WriteString("\n")
	}

	if len(dep.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range dep.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

// ---------------------------------------------------------------------------
// YAML get/update functions for edit support
// ---------------------------------------------------------------------------

func getPodYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	pod, err := client.DescribePod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	pod.ManagedFields = nil
	return yaml.Marshal(pod)
}

func updatePod(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var pod v1.Pod
	if err := yaml.Unmarshal(data, &pod); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdatePod(ctx, namespace, &pod)
}

func getServiceYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	svc, err := client.GetService(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	svc.ManagedFields = nil
	return yaml.Marshal(svc)
}

func updateService(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var svc v1.Service
	if err := yaml.Unmarshal(data, &svc); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateService(ctx, namespace, &svc)
}

func getConfigMapYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	cm, err := client.GetConfigMap(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	cm.ManagedFields = nil
	return yaml.Marshal(cm)
}

func updateConfigMap(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var cm v1.ConfigMap
	if err := yaml.Unmarshal(data, &cm); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateConfigMap(ctx, namespace, &cm)
}

func getSecretYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	secret, err := client.GetSecret(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	secret.ManagedFields = nil
	return yaml.Marshal(secret)
}

func updateSecret(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var secret v1.Secret
	if err := yaml.Unmarshal(data, &secret); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateSecret(ctx, namespace, &secret)
}

func getDeploymentYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	dep, err := client.GetDeployment(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	dep.ManagedFields = nil
	return yaml.Marshal(dep)
}

func updateDeployment(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var dep appsv1.Deployment
	if err := yaml.Unmarshal(data, &dep); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateDeployment(ctx, namespace, &dep)
}

// ---------------------------------------------------------------------------
// Delete functions for base resources
// ---------------------------------------------------------------------------

func deletePod(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeletePod(ctx, namespace, name)
}

func deleteService(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteService(ctx, namespace, name)
}

func deleteConfigMap(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteConfigMap(ctx, namespace, name)
}

func deleteSecret(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteSecret(ctx, namespace, name)
}

func deleteDeployment(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteDeployment(ctx, namespace, name)
}

// ---------------------------------------------------------------------------
// Helpers (migrated from pods.go)
// ---------------------------------------------------------------------------

func podReadyCount(pod v1.Pod) string {
	ready := 0
	total := len(pod.Spec.Containers)
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return fmt.Sprintf("%d/%d", ready, total)
}

func podRestartCount(pod v1.Pod) int32 {
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		restarts += cs.RestartCount
	}
	return restarts
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func podStatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running":
		return statusRunningStyle
	case "Pending":
		return statusPendingStyle
	case "Failed":
		return statusFailedStyle
	default:
		return statusOtherStyle
	}
}

func formatServicePorts(ports []v1.ServicePort) string {
	if len(ports) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		s := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.NodePort != 0 {
			s = fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, p.Protocol)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}
