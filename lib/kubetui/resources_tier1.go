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
	"time"

	"github.com/charmbracelet/lipgloss"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

// ---------------------------------------------------------------------------
// StatefulSets
// ---------------------------------------------------------------------------

var ResourceStatefulSets = &ResourceType{
	Name:       "StatefulSets",
	Command:    "sts",
	Aliases:    []string{"statefulsets", "statefulset"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "READY", Width: 9},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchStatefulSets,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeStatefulSet,
	GetYAMLFunc:  getStatefulSetYAML,
	UpdateFunc:   updateStatefulSet,
	DeleteFunc:   deleteStatefulSet,
}

func fetchStatefulSets(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListStatefulSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, s := range items {
		desired := int32(1)
		if s.Spec.Replicas != nil {
			desired = *s.Spec.Replicas
		}
		resources = append(resources, Resource{
			Namespace: s.Namespace,
			Name:      s.Name,
			Cells: []string{
				fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, desired),
				formatAge(s.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeStatefulSet(ctx context.Context, client *Client, namespace, name string) (string, error) {
	s, err := client.GetStatefulSet(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderStatefulSetDescribe(s), nil
}

func renderStatefulSetDescribe(s *appsv1.StatefulSet) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + s.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + s.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + s.CreationTimestamp.String() + "\n")
	desired := int32(1)
	if s.Spec.Replicas != nil {
		desired = *s.Spec.Replicas
	}
	b.WriteString(labelStyle.Render("Replicas:        ") + fmt.Sprintf("%d desired | %d total | %d ready | %d updated",
		desired, s.Status.Replicas, s.Status.ReadyReplicas, s.Status.UpdatedReplicas) + "\n")
	b.WriteString(labelStyle.Render("Update Strategy: ") + string(s.Spec.UpdateStrategy.Type) + "\n")
	if s.Spec.ServiceName != "" {
		b.WriteString(labelStyle.Render("Service Name:    ") + s.Spec.ServiceName + "\n")
	}
	b.WriteString(labelStyle.Render("Pod Management:  ") + string(s.Spec.PodManagementPolicy) + "\n")
	b.WriteString("\n")

	if len(s.Spec.Selector.MatchLabels) > 0 {
		b.WriteString(sectionStyle.Render("Selector") + "\n")
		for k, v := range s.Spec.Selector.MatchLabels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, s.Labels, s.Annotations)
	renderPodTemplate(&b, labelStyle, sectionStyle, s.Spec.Template)

	if len(s.Spec.VolumeClaimTemplates) > 0 {
		b.WriteString(sectionStyle.Render("Volume Claim Templates") + "\n")
		for _, vct := range s.Spec.VolumeClaimTemplates {
			b.WriteString(fmt.Sprintf("  %s\n", vct.Name))
			if vct.Spec.StorageClassName != nil {
				b.WriteString(fmt.Sprintf("    StorageClass:  %s\n", *vct.Spec.StorageClassName))
			}
			b.WriteString(fmt.Sprintf("    Access Modes:  %s\n", formatAccessModes(vct.Spec.AccessModes)))
			if storage, ok := vct.Spec.Resources.Requests[v1.ResourceStorage]; ok {
				b.WriteString(fmt.Sprintf("    Capacity:      %s\n", storage.String()))
			}
		}
		b.WriteString("\n")
	}

	if len(s.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range s.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

func getStatefulSetYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetStatefulSet(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateStatefulSet(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj appsv1.StatefulSet
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateStatefulSet(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// DaemonSets
// ---------------------------------------------------------------------------

var ResourceDaemonSets = &ResourceType{
	Name:       "DaemonSets",
	Command:    "ds",
	Aliases:    []string{"daemonsets", "daemonset"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "DESIRED", Width: 9},
		{Header: "CURRENT", Width: 9},
		{Header: "READY", Width: 9},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchDaemonSets,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeDaemonSet,
	GetYAMLFunc:  getDaemonSetYAML,
	UpdateFunc:   updateDaemonSet,
	DeleteFunc:   deleteDaemonSet,
}

func fetchDaemonSets(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListDaemonSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, d := range items {
		resources = append(resources, Resource{
			Namespace: d.Namespace,
			Name:      d.Name,
			Cells: []string{
				fmt.Sprintf("%d", d.Status.DesiredNumberScheduled),
				fmt.Sprintf("%d", d.Status.CurrentNumberScheduled),
				fmt.Sprintf("%d", d.Status.NumberReady),
				formatAge(d.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeDaemonSet(ctx context.Context, client *Client, namespace, name string) (string, error) {
	ds, err := client.GetDaemonSet(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderDaemonSetDescribe(ds), nil
}

func renderDaemonSetDescribe(ds *appsv1.DaemonSet) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + ds.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + ds.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + ds.CreationTimestamp.String() + "\n")
	b.WriteString(labelStyle.Render("Desired:         ") + fmt.Sprintf("%d", ds.Status.DesiredNumberScheduled) + "\n")
	b.WriteString(labelStyle.Render("Current:         ") + fmt.Sprintf("%d", ds.Status.CurrentNumberScheduled) + "\n")
	b.WriteString(labelStyle.Render("Ready:           ") + fmt.Sprintf("%d", ds.Status.NumberReady) + "\n")
	b.WriteString(labelStyle.Render("Up-to-date:      ") + fmt.Sprintf("%d", ds.Status.UpdatedNumberScheduled) + "\n")
	b.WriteString(labelStyle.Render("Available:       ") + fmt.Sprintf("%d", ds.Status.NumberAvailable) + "\n")
	b.WriteString(labelStyle.Render("Update Strategy: ") + string(ds.Spec.UpdateStrategy.Type) + "\n")
	if ds.Status.NumberMisscheduled > 0 {
		b.WriteString(labelStyle.Render("Misscheduled:    ") + fmt.Sprintf("%d", ds.Status.NumberMisscheduled) + "\n")
	}
	b.WriteString("\n")

	if ds.Spec.Selector != nil && len(ds.Spec.Selector.MatchLabels) > 0 {
		b.WriteString(sectionStyle.Render("Selector") + "\n")
		for k, v := range ds.Spec.Selector.MatchLabels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, ds.Labels, ds.Annotations)
	renderPodTemplate(&b, labelStyle, sectionStyle, ds.Spec.Template)

	if len(ds.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range ds.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

func getDaemonSetYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetDaemonSet(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateDaemonSet(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj appsv1.DaemonSet
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateDaemonSet(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// ReplicaSets
// ---------------------------------------------------------------------------

var ResourceReplicaSets = &ResourceType{
	Name:       "ReplicaSets",
	Command:    "rs",
	Aliases:    []string{"replicasets", "replicaset"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "DESIRED", Width: 9},
		{Header: "CURRENT", Width: 9},
		{Header: "READY", Width: 9},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchReplicaSets,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeReplicaSet,
	GetYAMLFunc:  getReplicaSetYAML,
	UpdateFunc:   updateReplicaSet,
	DeleteFunc:   deleteReplicaSet,
}

func fetchReplicaSets(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListReplicaSets(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, r := range items {
		desired := int32(0)
		if r.Spec.Replicas != nil {
			desired = *r.Spec.Replicas
		}
		resources = append(resources, Resource{
			Namespace: r.Namespace,
			Name:      r.Name,
			Cells: []string{
				fmt.Sprintf("%d", desired),
				fmt.Sprintf("%d", r.Status.Replicas),
				fmt.Sprintf("%d", r.Status.ReadyReplicas),
				formatAge(r.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeReplicaSet(ctx context.Context, client *Client, namespace, name string) (string, error) {
	rs, err := client.GetReplicaSet(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderReplicaSetDescribe(rs), nil
}

func renderReplicaSetDescribe(rs *appsv1.ReplicaSet) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + rs.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + rs.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + rs.CreationTimestamp.String() + "\n")
	desired := int32(0)
	if rs.Spec.Replicas != nil {
		desired = *rs.Spec.Replicas
	}
	b.WriteString(labelStyle.Render("Replicas:        ") + fmt.Sprintf("%d desired | %d total | %d ready | %d available",
		desired, rs.Status.Replicas, rs.Status.ReadyReplicas, rs.Status.AvailableReplicas) + "\n")
	b.WriteString("\n")

	if rs.Spec.Selector != nil && len(rs.Spec.Selector.MatchLabels) > 0 {
		b.WriteString(sectionStyle.Render("Selector") + "\n")
		for k, v := range rs.Spec.Selector.MatchLabels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, rs.Labels, rs.Annotations)
	renderPodTemplate(&b, labelStyle, sectionStyle, rs.Spec.Template)

	if len(rs.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range rs.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

func getReplicaSetYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetReplicaSet(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateReplicaSet(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj appsv1.ReplicaSet
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateReplicaSet(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

var ResourceJobs = &ResourceType{
	Name:       "Jobs",
	Command:    "jobs",
	Aliases:    []string{"job", "jo"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "COMPLETIONS", Width: 13},
		{Header: "DURATION", Width: 10},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchJobs,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeJob,
	GetYAMLFunc:  getJobYAML,
	UpdateFunc:   updateJob,
	DeleteFunc:   deleteJob,
}

func fetchJobs(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListJobs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, j := range items {
		desired := int32(1)
		if j.Spec.Completions != nil {
			desired = *j.Spec.Completions
		}
		duration := "<pending>"
		if j.Status.StartTime != nil {
			end := time.Now()
			if j.Status.CompletionTime != nil {
				end = j.Status.CompletionTime.Time
			}
			duration = formatAge(end.Add(-end.Sub(j.Status.StartTime.Time)))
		}
		resources = append(resources, Resource{
			Namespace: j.Namespace,
			Name:      j.Name,
			Cells: []string{
				fmt.Sprintf("%d/%d", j.Status.Succeeded, desired),
				duration,
				formatAge(j.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeJob(ctx context.Context, client *Client, namespace, name string) (string, error) {
	job, err := client.GetJob(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderJobDescribe(job), nil
}

func renderJobDescribe(job *batchv1.Job) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + job.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + job.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + job.CreationTimestamp.String() + "\n")

	completions := int32(1)
	if job.Spec.Completions != nil {
		completions = *job.Spec.Completions
	}
	parallelism := int32(1)
	if job.Spec.Parallelism != nil {
		parallelism = *job.Spec.Parallelism
	}
	b.WriteString(labelStyle.Render("Completions:     ") + fmt.Sprintf("%d/%d", job.Status.Succeeded, completions) + "\n")
	b.WriteString(labelStyle.Render("Parallelism:     ") + fmt.Sprintf("%d", parallelism) + "\n")
	if job.Spec.BackoffLimit != nil {
		b.WriteString(labelStyle.Render("Backoff Limit:   ") + fmt.Sprintf("%d", *job.Spec.BackoffLimit) + "\n")
	}
	if job.Status.StartTime != nil {
		b.WriteString(labelStyle.Render("Start Time:      ") + job.Status.StartTime.String() + "\n")
	}
	if job.Status.CompletionTime != nil {
		b.WriteString(labelStyle.Render("Completed:       ") + job.Status.CompletionTime.String() + "\n")
		if job.Status.StartTime != nil {
			duration := job.Status.CompletionTime.Time.Sub(job.Status.StartTime.Time)
			b.WriteString(labelStyle.Render("Duration:        ") + duration.String() + "\n")
		}
	}
	b.WriteString(labelStyle.Render("Active:          ") + fmt.Sprintf("%d", job.Status.Active) + "\n")
	b.WriteString(labelStyle.Render("Succeeded:       ") + fmt.Sprintf("%d", job.Status.Succeeded) + "\n")
	b.WriteString(labelStyle.Render("Failed:          ") + fmt.Sprintf("%d", job.Status.Failed) + "\n")
	b.WriteString("\n")

	if job.Spec.Selector != nil && len(job.Spec.Selector.MatchLabels) > 0 {
		b.WriteString(sectionStyle.Render("Selector") + "\n")
		for k, v := range job.Spec.Selector.MatchLabels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, job.Labels, job.Annotations)
	renderPodTemplate(&b, labelStyle, sectionStyle, job.Spec.Template)

	if len(job.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range job.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

func getJobYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetJob(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateJob(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj batchv1.Job
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateJob(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// CronJobs
// ---------------------------------------------------------------------------

var ResourceCronJobs = &ResourceType{
	Name:       "CronJobs",
	Command:    "cj",
	Aliases:    []string{"cronjobs", "cronjob"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "SCHEDULE", Width: 20},
		{Header: "SUSPEND", Width: 9},
		{Header: "LAST SCHEDULE", Width: 15},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchCronJobs,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeCronJob,
	GetYAMLFunc:  getCronJobYAML,
	UpdateFunc:   updateCronJob,
	DeleteFunc:   deleteCronJob,
}

func fetchCronJobs(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListCronJobs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, cj := range items {
		suspend := "False"
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			suspend = "True"
		}
		lastSchedule := "<none>"
		if cj.Status.LastScheduleTime != nil {
			lastSchedule = formatAge(cj.Status.LastScheduleTime.Time)
		}
		resources = append(resources, Resource{
			Namespace: cj.Namespace,
			Name:      cj.Name,
			Cells: []string{
				cj.Spec.Schedule,
				suspend,
				lastSchedule,
				formatAge(cj.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeCronJob(ctx context.Context, client *Client, namespace, name string) (string, error) {
	cj, err := client.GetCronJob(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderCronJobDescribe(cj), nil
}

func renderCronJobDescribe(cj *batchv1.CronJob) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:              ") + cj.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:         ") + cj.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:      ") + cj.CreationTimestamp.String() + "\n")
	b.WriteString(labelStyle.Render("Schedule:          ") + cj.Spec.Schedule + "\n")
	suspend := "False"
	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		suspend = "True"
	}
	b.WriteString(labelStyle.Render("Suspend:           ") + suspend + "\n")
	b.WriteString(labelStyle.Render("Concurrency Policy:") + " " + string(cj.Spec.ConcurrencyPolicy) + "\n")
	if cj.Spec.StartingDeadlineSeconds != nil {
		b.WriteString(labelStyle.Render("Starting Deadline: ") + fmt.Sprintf("%ds", *cj.Spec.StartingDeadlineSeconds) + "\n")
	}
	if cj.Spec.SuccessfulJobsHistoryLimit != nil {
		b.WriteString(labelStyle.Render("Success History:   ") + fmt.Sprintf("%d", *cj.Spec.SuccessfulJobsHistoryLimit) + "\n")
	}
	if cj.Spec.FailedJobsHistoryLimit != nil {
		b.WriteString(labelStyle.Render("Failed History:    ") + fmt.Sprintf("%d", *cj.Spec.FailedJobsHistoryLimit) + "\n")
	}
	if cj.Status.LastScheduleTime != nil {
		b.WriteString(labelStyle.Render("Last Schedule:     ") + cj.Status.LastScheduleTime.String() + "\n")
	}
	if cj.Status.LastSuccessfulTime != nil {
		b.WriteString(labelStyle.Render("Last Success:      ") + cj.Status.LastSuccessfulTime.String() + "\n")
	}
	b.WriteString(labelStyle.Render("Active Jobs:       ") + fmt.Sprintf("%d", len(cj.Status.Active)) + "\n")
	b.WriteString("\n")

	renderLabelsAndAnnotations(&b, sectionStyle, cj.Labels, cj.Annotations)

	// Show job template's pod template
	b.WriteString(sectionStyle.Render("Job Template") + "\n")
	b.WriteString(fmt.Sprintf("  Completions:  %s\n", ptrInt32Str(cj.Spec.JobTemplate.Spec.Completions, "1")))
	b.WriteString(fmt.Sprintf("  Parallelism:  %s\n", ptrInt32Str(cj.Spec.JobTemplate.Spec.Parallelism, "1")))
	b.WriteString("\n")
	renderPodTemplate(&b, labelStyle, sectionStyle, cj.Spec.JobTemplate.Spec.Template)

	return b.String()
}

func getCronJobYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetCronJob(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateCronJob(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj batchv1.CronJob
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateCronJob(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// Nodes (cluster-scoped)
// ---------------------------------------------------------------------------

var ResourceNodes = &ResourceType{
	Name:       "Nodes",
	Command:    "nodes",
	Aliases:    []string{"node", "no"},
	Namespaced: false,
	Columns: []Column{
		{Header: "NAME", Flexible: true},
		{Header: "STATUS", Width: 12},
		{Header: "ROLES", Width: 20},
		{Header: "AGE", Width: 10},
		{Header: "VERSION", Width: 16},
	},
	FetchFunc:    fetchNodes,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeNode,
	GetYAMLFunc:  getNodeYAML,
	UpdateFunc:   updateNode,
	DeleteFunc:   deleteNode,
}

func fetchNodes(ctx context.Context, client *Client, _ string) ([]Resource, error) {
	items, err := client.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, n := range items {
		status := "NotReady"
		for _, c := range n.Status.Conditions {
			if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
				status = "Ready"
				break
			}
		}
		roles := nodeRoles(n)
		resources = append(resources, Resource{
			Name: n.Name,
			Cells: []string{
				status,
				roles,
				formatAge(n.CreationTimestamp.Time),
				n.Status.NodeInfo.KubeletVersion,
			},
		})
	}
	return resources, nil
}

func nodeRoles(node v1.Node) string {
	var roles []string
	for label := range node.Labels {
		const prefix = "node-role.kubernetes.io/"
		if strings.HasPrefix(label, prefix) {
			role := strings.TrimPrefix(label, prefix)
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

func describeNode(ctx context.Context, client *Client, _, name string) (string, error) {
	node, err := client.GetNode(ctx, name)
	if err != nil {
		return "", err
	}
	return renderNodeDescribe(node), nil
}

func renderNodeDescribe(node *v1.Node) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	status := "NotReady"
	for _, c := range node.Status.Conditions {
		if c.Type == v1.NodeReady && c.Status == v1.ConditionTrue {
			status = "Ready"
			break
		}
	}

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:              ") + node.Name + "\n")
	b.WriteString(labelStyle.Render("Status:            ") + status + "\n")
	b.WriteString(labelStyle.Render("Roles:             ") + nodeRoles(*node) + "\n")
	b.WriteString(labelStyle.Render("CreationTime:      ") + node.CreationTimestamp.String() + "\n")
	b.WriteString(labelStyle.Render("OS Image:          ") + node.Status.NodeInfo.OSImage + "\n")
	b.WriteString(labelStyle.Render("Kernel Version:    ") + node.Status.NodeInfo.KernelVersion + "\n")
	b.WriteString(labelStyle.Render("Container Runtime: ") + node.Status.NodeInfo.ContainerRuntimeVersion + "\n")
	b.WriteString(labelStyle.Render("Kubelet Version:   ") + node.Status.NodeInfo.KubeletVersion + "\n")
	b.WriteString(labelStyle.Render("Kube-Proxy:        ") + node.Status.NodeInfo.KubeProxyVersion + "\n")
	b.WriteString(labelStyle.Render("Architecture:      ") + node.Status.NodeInfo.Architecture + "\n")
	b.WriteString(labelStyle.Render("Operating System:  ") + node.Status.NodeInfo.OperatingSystem + "\n")
	if node.Spec.PodCIDR != "" {
		b.WriteString(labelStyle.Render("Pod CIDR:          ") + node.Spec.PodCIDR + "\n")
	}
	if node.Spec.ProviderID != "" {
		b.WriteString(labelStyle.Render("Provider ID:       ") + node.Spec.ProviderID + "\n")
	}
	if node.Spec.Unschedulable {
		b.WriteString(labelStyle.Render("Unschedulable:     ") + "true\n")
	}
	b.WriteString("\n")

	if len(node.Status.Addresses) > 0 {
		b.WriteString(sectionStyle.Render("Addresses") + "\n")
		for _, addr := range node.Status.Addresses {
			b.WriteString(fmt.Sprintf("  %-15s %s\n", addr.Type, addr.Address))
		}
		b.WriteString("\n")
	}

	b.WriteString(sectionStyle.Render("Capacity") + "\n")
	for res, qty := range node.Status.Capacity {
		b.WriteString(fmt.Sprintf("  %-20s %s\n", res, qty.String()))
	}
	b.WriteString("\n")

	b.WriteString(sectionStyle.Render("Allocatable") + "\n")
	for res, qty := range node.Status.Allocatable {
		b.WriteString(fmt.Sprintf("  %-20s %s\n", res, qty.String()))
	}
	b.WriteString("\n")

	renderLabelsAndAnnotations(&b, sectionStyle, node.Labels, node.Annotations)

	if len(node.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range node.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %-6s %s\n", c.Type, string(c.Status), c.Message))
		}
		b.WriteString("\n")
	}

	if len(node.Spec.Taints) > 0 {
		b.WriteString(sectionStyle.Render("Taints") + "\n")
		for _, t := range node.Spec.Taints {
			b.WriteString(fmt.Sprintf("  %s=%s:%s\n", t.Key, t.Value, t.Effect))
		}
	}

	return b.String()
}

func getNodeYAML(ctx context.Context, client *Client, _, name string) ([]byte, error) {
	obj, err := client.GetNode(ctx, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateNode(ctx context.Context, client *Client, _, _ string, data []byte) error {
	var obj v1.Node
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateNode(ctx, &obj)
}

// ---------------------------------------------------------------------------
// PersistentVolumes (cluster-scoped)
// ---------------------------------------------------------------------------

var ResourcePersistentVolumes = &ResourceType{
	Name:       "PersistentVolumes",
	Command:    "pv",
	Aliases:    []string{"persistentvolumes", "persistentvolume"},
	Namespaced: false,
	Columns: []Column{
		{Header: "NAME", Flexible: true},
		{Header: "CAPACITY", Width: 12},
		{Header: "ACCESS MODES", Width: 14},
		{Header: "RECLAIM POLICY", Width: 16},
		{Header: "STATUS", Width: 10},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchPersistentVolumes,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describePersistentVolume,
	GetYAMLFunc:  getPersistentVolumeYAML,
	UpdateFunc:   updatePersistentVolume,
	DeleteFunc:   deletePersistentVolume,
}

func fetchPersistentVolumes(ctx context.Context, client *Client, _ string) ([]Resource, error) {
	items, err := client.ListPersistentVolumes(ctx)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, pv := range items {
		capacity := ""
		if storage, ok := pv.Spec.Capacity[v1.ResourceStorage]; ok {
			capacity = storage.String()
		}
		resources = append(resources, Resource{
			Name: pv.Name,
			Cells: []string{
				capacity,
				formatAccessModes(pv.Spec.AccessModes),
				string(pv.Spec.PersistentVolumeReclaimPolicy),
				string(pv.Status.Phase),
				formatAge(pv.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func formatAccessModes(modes []v1.PersistentVolumeAccessMode) string {
	abbrevs := make([]string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case v1.ReadWriteOnce:
			abbrevs = append(abbrevs, "RWO")
		case v1.ReadOnlyMany:
			abbrevs = append(abbrevs, "ROX")
		case v1.ReadWriteMany:
			abbrevs = append(abbrevs, "RWX")
		case v1.ReadWriteOncePod:
			abbrevs = append(abbrevs, "RWOP")
		default:
			abbrevs = append(abbrevs, string(m))
		}
	}
	return strings.Join(abbrevs, ",")
}

func describePersistentVolume(ctx context.Context, client *Client, _, name string) (string, error) {
	pv, err := client.GetPersistentVolume(ctx, name)
	if err != nil {
		return "", err
	}
	return renderPersistentVolumeDescribe(pv), nil
}

func renderPersistentVolumeDescribe(pv *v1.PersistentVolume) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:              ") + pv.Name + "\n")
	b.WriteString(labelStyle.Render("CreationTime:      ") + pv.CreationTimestamp.String() + "\n")
	b.WriteString(labelStyle.Render("Status:            ") + string(pv.Status.Phase) + "\n")
	if storage, ok := pv.Spec.Capacity[v1.ResourceStorage]; ok {
		b.WriteString(labelStyle.Render("Capacity:          ") + storage.String() + "\n")
	}
	b.WriteString(labelStyle.Render("Access Modes:      ") + formatAccessModes(pv.Spec.AccessModes) + "\n")
	b.WriteString(labelStyle.Render("Reclaim Policy:    ") + string(pv.Spec.PersistentVolumeReclaimPolicy) + "\n")
	if pv.Spec.StorageClassName != "" {
		b.WriteString(labelStyle.Render("Storage Class:     ") + pv.Spec.StorageClassName + "\n")
	}
	if pv.Spec.VolumeMode != nil {
		b.WriteString(labelStyle.Render("Volume Mode:       ") + string(*pv.Spec.VolumeMode) + "\n")
	}
	if pv.Spec.ClaimRef != nil {
		b.WriteString(labelStyle.Render("Claim:             ") + fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name) + "\n")
	}
	if pv.Status.Reason != "" {
		b.WriteString(labelStyle.Render("Reason:            ") + pv.Status.Reason + "\n")
	}
	b.WriteString("\n")

	// Volume source
	b.WriteString(sectionStyle.Render("Source") + "\n")
	switch {
	case pv.Spec.HostPath != nil:
		b.WriteString(fmt.Sprintf("  Type:    HostPath\n  Path:    %s\n", pv.Spec.HostPath.Path))
	case pv.Spec.NFS != nil:
		b.WriteString(fmt.Sprintf("  Type:    NFS\n  Server:  %s\n  Path:    %s\n", pv.Spec.NFS.Server, pv.Spec.NFS.Path))
	case pv.Spec.CSI != nil:
		b.WriteString(fmt.Sprintf("  Type:    CSI\n  Driver:  %s\n  Handle:  %s\n", pv.Spec.CSI.Driver, pv.Spec.CSI.VolumeHandle))
	case pv.Spec.Local != nil:
		b.WriteString(fmt.Sprintf("  Type:    Local\n  Path:    %s\n", pv.Spec.Local.Path))
	default:
		b.WriteString("  <unknown>\n")
	}
	b.WriteString("\n")

	renderLabelsAndAnnotations(&b, sectionStyle, pv.Labels, pv.Annotations)

	return b.String()
}

func getPersistentVolumeYAML(ctx context.Context, client *Client, _, name string) ([]byte, error) {
	obj, err := client.GetPersistentVolume(ctx, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updatePersistentVolume(ctx context.Context, client *Client, _, _ string, data []byte) error {
	var obj v1.PersistentVolume
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdatePersistentVolume(ctx, &obj)
}

// ---------------------------------------------------------------------------
// PersistentVolumeClaims
// ---------------------------------------------------------------------------

var ResourcePersistentVolumeClaims = &ResourceType{
	Name:       "PersistentVolumeClaims",
	Command:    "pvc",
	Aliases:    []string{"persistentvolumeclaims", "persistentvolumeclaim"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "STATUS", Width: 10},
		{Header: "VOLUME", Width: 30},
		{Header: "CAPACITY", Width: 12},
		{Header: "STORAGE CLASS", Width: 20},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchPersistentVolumeClaims,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describePersistentVolumeClaim,
	GetYAMLFunc:  getPersistentVolumeClaimYAML,
	UpdateFunc:   updatePersistentVolumeClaim,
	DeleteFunc:   deletePersistentVolumeClaim,
}

func fetchPersistentVolumeClaims(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListPersistentVolumeClaims(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, pvc := range items {
		capacity := ""
		if pvc.Status.Capacity != nil {
			if storage, ok := pvc.Status.Capacity[v1.ResourceStorage]; ok {
				capacity = storage.String()
			}
		}
		storageClass := ""
		if pvc.Spec.StorageClassName != nil {
			storageClass = *pvc.Spec.StorageClassName
		}
		resources = append(resources, Resource{
			Namespace: pvc.Namespace,
			Name:      pvc.Name,
			Cells: []string{
				string(pvc.Status.Phase),
				pvc.Spec.VolumeName,
				capacity,
				storageClass,
				formatAge(pvc.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describePersistentVolumeClaim(ctx context.Context, client *Client, namespace, name string) (string, error) {
	pvc, err := client.GetPersistentVolumeClaim(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderPVCDescribe(pvc), nil
}

func renderPVCDescribe(pvc *v1.PersistentVolumeClaim) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + pvc.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + pvc.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + pvc.CreationTimestamp.String() + "\n")
	b.WriteString(labelStyle.Render("Status:          ") + string(pvc.Status.Phase) + "\n")
	b.WriteString(labelStyle.Render("Volume:          ") + pvc.Spec.VolumeName + "\n")
	if pvc.Status.Capacity != nil {
		if storage, ok := pvc.Status.Capacity[v1.ResourceStorage]; ok {
			b.WriteString(labelStyle.Render("Capacity:        ") + storage.String() + "\n")
		}
	}
	b.WriteString(labelStyle.Render("Access Modes:    ") + formatAccessModes(pvc.Spec.AccessModes) + "\n")
	if pvc.Spec.StorageClassName != nil {
		b.WriteString(labelStyle.Render("Storage Class:   ") + *pvc.Spec.StorageClassName + "\n")
	}
	if pvc.Spec.VolumeMode != nil {
		b.WriteString(labelStyle.Render("Volume Mode:     ") + string(*pvc.Spec.VolumeMode) + "\n")
	}
	b.WriteString("\n")

	renderLabelsAndAnnotations(&b, sectionStyle, pvc.Labels, pvc.Annotations)

	if len(pvc.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range pvc.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

func getPersistentVolumeClaimYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetPersistentVolumeClaim(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updatePersistentVolumeClaim(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj v1.PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdatePersistentVolumeClaim(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// ServiceAccounts
// ---------------------------------------------------------------------------

var ResourceServiceAccounts = &ResourceType{
	Name:       "ServiceAccounts",
	Command:    "sa",
	Aliases:    []string{"serviceaccounts", "serviceaccount"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "SECRETS", Width: 9},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchServiceAccounts,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeServiceAccount,
	GetYAMLFunc:  getServiceAccountYAML,
	UpdateFunc:   updateServiceAccount,
	DeleteFunc:   deleteServiceAccount,
}

func fetchServiceAccounts(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListServiceAccounts(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, sa := range items {
		resources = append(resources, Resource{
			Namespace: sa.Namespace,
			Name:      sa.Name,
			Cells: []string{
				fmt.Sprintf("%d", len(sa.Secrets)),
				formatAge(sa.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeServiceAccount(ctx context.Context, client *Client, namespace, name string) (string, error) {
	sa, err := client.GetServiceAccount(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderServiceAccountDescribe(sa), nil
}

func renderServiceAccountDescribe(sa *v1.ServiceAccount) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + sa.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + sa.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + sa.CreationTimestamp.String() + "\n")
	b.WriteString("\n")

	if len(sa.Secrets) > 0 {
		b.WriteString(sectionStyle.Render("Secrets") + "\n")
		for _, s := range sa.Secrets {
			b.WriteString(fmt.Sprintf("  %s\n", s.Name))
		}
		b.WriteString("\n")
	}

	if len(sa.ImagePullSecrets) > 0 {
		b.WriteString(sectionStyle.Render("Image Pull Secrets") + "\n")
		for _, s := range sa.ImagePullSecrets {
			b.WriteString(fmt.Sprintf("  %s\n", s.Name))
		}
		b.WriteString("\n")
	}

	if sa.AutomountServiceAccountToken != nil {
		b.WriteString(labelStyle.Render("Automount Token: ") + fmt.Sprintf("%t", *sa.AutomountServiceAccountToken) + "\n\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, sa.Labels, sa.Annotations)

	return b.String()
}

func getServiceAccountYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetServiceAccount(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateServiceAccount(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj v1.ServiceAccount
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateServiceAccount(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// Ingresses
// ---------------------------------------------------------------------------

var ResourceIngresses = &ResourceType{
	Name:       "Ingresses",
	Command:    "ing",
	Aliases:    []string{"ingresses", "ingress"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "CLASS", Width: 16},
		{Header: "HOSTS", Width: 30},
		{Header: "ADDRESS", Width: 20},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchIngresses,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeIngress,
	GetYAMLFunc:  getIngressYAML,
	UpdateFunc:   updateIngress,
	DeleteFunc:   deleteIngress,
}

func fetchIngresses(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListIngresses(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, ing := range items {
		class := ""
		if ing.Spec.IngressClassName != nil {
			class = *ing.Spec.IngressClassName
		}
		hosts := ingressHosts(ing)
		address := ingressAddress(ing)
		resources = append(resources, Resource{
			Namespace: ing.Namespace,
			Name:      ing.Name,
			Cells: []string{
				class,
				hosts,
				address,
				formatAge(ing.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func ingressHosts(ing networkingv1.Ingress) string {
	hosts := make([]string, 0)
	for _, rule := range ing.Spec.Rules {
		if rule.Host != "" {
			hosts = append(hosts, rule.Host)
		}
	}
	if len(hosts) == 0 {
		return "*"
	}
	return strings.Join(hosts, ",")
}

func ingressAddress(ing networkingv1.Ingress) string {
	addrs := make([]string, 0)
	for _, lb := range ing.Status.LoadBalancer.Ingress {
		if lb.IP != "" {
			addrs = append(addrs, lb.IP)
		} else if lb.Hostname != "" {
			addrs = append(addrs, lb.Hostname)
		}
	}
	return strings.Join(addrs, ",")
}

func describeIngress(ctx context.Context, client *Client, namespace, name string) (string, error) {
	ing, err := client.GetIngress(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderIngressDescribe(ing), nil
}

func renderIngressDescribe(ing *networkingv1.Ingress) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + ing.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + ing.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + ing.CreationTimestamp.String() + "\n")
	if ing.Spec.IngressClassName != nil {
		b.WriteString(labelStyle.Render("Class:           ") + *ing.Spec.IngressClassName + "\n")
	}
	address := ingressAddress(*ing)
	if address != "" {
		b.WriteString(labelStyle.Render("Address:         ") + address + "\n")
	}
	b.WriteString("\n")

	if ing.Spec.DefaultBackend != nil {
		b.WriteString(sectionStyle.Render("Default Backend") + "\n")
		if ing.Spec.DefaultBackend.Service != nil {
			svc := ing.Spec.DefaultBackend.Service
			port := ""
			if svc.Port.Name != "" {
				port = svc.Port.Name
			} else {
				port = fmt.Sprintf("%d", svc.Port.Number)
			}
			b.WriteString(fmt.Sprintf("  %s:%s\n", svc.Name, port))
		}
		b.WriteString("\n")
	}

	if len(ing.Spec.Rules) > 0 {
		b.WriteString(sectionStyle.Render("Rules") + "\n")
		for _, rule := range ing.Spec.Rules {
			host := rule.Host
			if host == "" {
				host = "*"
			}
			b.WriteString(labelStyle.Render("  Host: ") + host + "\n")
			if rule.HTTP != nil {
				for _, path := range rule.HTTP.Paths {
					pathStr := "/"
					if path.Path != "" {
						pathStr = path.Path
					}
					pathType := ""
					if path.PathType != nil {
						pathType = string(*path.PathType)
					}
					backend := ""
					if path.Backend.Service != nil {
						svc := path.Backend.Service
						port := ""
						if svc.Port.Name != "" {
							port = svc.Port.Name
						} else {
							port = fmt.Sprintf("%d", svc.Port.Number)
						}
						backend = fmt.Sprintf("%s:%s", svc.Name, port)
					}
					b.WriteString(fmt.Sprintf("    %-30s %-15s %s\n", pathStr, pathType, backend))
				}
			}
		}
		b.WriteString("\n")
	}

	if len(ing.Spec.TLS) > 0 {
		b.WriteString(sectionStyle.Render("TLS") + "\n")
		for _, tls := range ing.Spec.TLS {
			b.WriteString(fmt.Sprintf("  Secret: %s\n", tls.SecretName))
			if len(tls.Hosts) > 0 {
				b.WriteString(fmt.Sprintf("  Hosts:  %s\n", strings.Join(tls.Hosts, ", ")))
			}
		}
		b.WriteString("\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, ing.Labels, ing.Annotations)

	return b.String()
}

func getIngressYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetIngress(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateIngress(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj networkingv1.Ingress
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateIngress(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// HorizontalPodAutoscalers
// ---------------------------------------------------------------------------

var ResourceHPAs = &ResourceType{
	Name:       "HorizontalPodAutoscalers",
	Command:    "hpa",
	Aliases:    []string{"horizontalpodautoscalers"},
	Namespaced: true,
	Columns: []Column{
		{Header: "NAMESPACE", Width: 20},
		{Header: "NAME", Flexible: true},
		{Header: "REFERENCE", Width: 30},
		{Header: "MINPODS", Width: 9},
		{Header: "MAXPODS", Width: 9},
		{Header: "REPLICAS", Width: 10},
		{Header: "AGE", Width: 10},
	},
	FetchFunc:    fetchHPAs,
	Actions:      []string{"describe", "edit", "delete"},
	DescribeFunc: describeHPA,
	GetYAMLFunc:  getHPAYAML,
	UpdateFunc:   updateHPA,
	DeleteFunc:   deleteHPA,
}

func fetchHPAs(ctx context.Context, client *Client, namespace string) ([]Resource, error) {
	items, err := client.ListHPAs(ctx, namespace)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, hpa := range items {
		minPods := int32(1)
		if hpa.Spec.MinReplicas != nil {
			minPods = *hpa.Spec.MinReplicas
		}
		ref := fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name)
		resources = append(resources, Resource{
			Namespace: hpa.Namespace,
			Name:      hpa.Name,
			Cells: []string{
				ref,
				fmt.Sprintf("%d", minPods),
				fmt.Sprintf("%d", hpa.Spec.MaxReplicas),
				fmt.Sprintf("%d", hpa.Status.CurrentReplicas),
				formatAge(hpa.CreationTimestamp.Time),
			},
		})
	}
	return resources, nil
}

func describeHPA(ctx context.Context, client *Client, namespace, name string) (string, error) {
	hpa, err := client.GetHPA(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return renderHPADescribe(hpa), nil
}

func renderHPADescribe(hpa *autoscalingv2.HorizontalPodAutoscaler) string {
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(primaryColor)
	sectionStyle := lipgloss.NewStyle().Bold(true).Foreground(secondaryColor).Underline(true)

	var b strings.Builder

	b.WriteString(sectionStyle.Render("General") + "\n")
	b.WriteString(labelStyle.Render("Name:            ") + hpa.Name + "\n")
	b.WriteString(labelStyle.Render("Namespace:       ") + hpa.Namespace + "\n")
	b.WriteString(labelStyle.Render("CreationTime:    ") + hpa.CreationTimestamp.String() + "\n")
	b.WriteString(labelStyle.Render("Reference:       ") + fmt.Sprintf("%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Spec.ScaleTargetRef.Name) + "\n")
	minPods := int32(1)
	if hpa.Spec.MinReplicas != nil {
		minPods = *hpa.Spec.MinReplicas
	}
	b.WriteString(labelStyle.Render("Min Replicas:    ") + fmt.Sprintf("%d", minPods) + "\n")
	b.WriteString(labelStyle.Render("Max Replicas:    ") + fmt.Sprintf("%d", hpa.Spec.MaxReplicas) + "\n")
	b.WriteString(labelStyle.Render("Current Replicas:") + " " + fmt.Sprintf("%d", hpa.Status.CurrentReplicas) + "\n")
	if hpa.Status.DesiredReplicas > 0 {
		b.WriteString(labelStyle.Render("Desired Replicas:") + " " + fmt.Sprintf("%d", hpa.Status.DesiredReplicas) + "\n")
	}
	b.WriteString("\n")

	if len(hpa.Spec.Metrics) > 0 {
		b.WriteString(sectionStyle.Render("Metrics") + "\n")
		for _, m := range hpa.Spec.Metrics {
			switch m.Type {
			case autoscalingv2.ResourceMetricSourceType:
				if m.Resource != nil {
					target := "<not set>"
					if m.Resource.Target.AverageUtilization != nil {
						target = fmt.Sprintf("%d%%", *m.Resource.Target.AverageUtilization)
					} else if m.Resource.Target.AverageValue != nil {
						target = m.Resource.Target.AverageValue.String()
					}
					// Find current value
					current := "<unknown>"
					for _, cs := range hpa.Status.CurrentMetrics {
						if cs.Type == autoscalingv2.ResourceMetricSourceType && cs.Resource != nil &&
							cs.Resource.Name == m.Resource.Name {
							if cs.Resource.Current.AverageUtilization != nil {
								current = fmt.Sprintf("%d%%", *cs.Resource.Current.AverageUtilization)
							} else if cs.Resource.Current.AverageValue != nil {
								current = cs.Resource.Current.AverageValue.String()
							}
						}
					}
					b.WriteString(fmt.Sprintf("  %s: %s / %s (target)\n", m.Resource.Name, current, target))
				}
			case autoscalingv2.PodsMetricSourceType:
				if m.Pods != nil {
					b.WriteString(fmt.Sprintf("  Pods metric: %s (target: %s)\n",
						m.Pods.Metric.Name, m.Pods.Target.AverageValue.String()))
				}
			case autoscalingv2.ObjectMetricSourceType:
				if m.Object != nil {
					b.WriteString(fmt.Sprintf("  Object metric: %s on %s/%s\n",
						m.Object.Metric.Name, m.Object.DescribedObject.Kind, m.Object.DescribedObject.Name))
				}
			default:
				b.WriteString(fmt.Sprintf("  %s metric\n", m.Type))
			}
		}
		b.WriteString("\n")
	}

	if hpa.Spec.Behavior != nil {
		b.WriteString(sectionStyle.Render("Behavior") + "\n")
		if hpa.Spec.Behavior.ScaleUp != nil {
			b.WriteString("  Scale Up:\n")
			if hpa.Spec.Behavior.ScaleUp.StabilizationWindowSeconds != nil {
				b.WriteString(fmt.Sprintf("    Stabilization Window: %ds\n", *hpa.Spec.Behavior.ScaleUp.StabilizationWindowSeconds))
			}
			for _, p := range hpa.Spec.Behavior.ScaleUp.Policies {
				b.WriteString(fmt.Sprintf("    %s: %d per %ds\n", p.Type, p.Value, p.PeriodSeconds))
			}
		}
		if hpa.Spec.Behavior.ScaleDown != nil {
			b.WriteString("  Scale Down:\n")
			if hpa.Spec.Behavior.ScaleDown.StabilizationWindowSeconds != nil {
				b.WriteString(fmt.Sprintf("    Stabilization Window: %ds\n", *hpa.Spec.Behavior.ScaleDown.StabilizationWindowSeconds))
			}
			for _, p := range hpa.Spec.Behavior.ScaleDown.Policies {
				b.WriteString(fmt.Sprintf("    %s: %d per %ds\n", p.Type, p.Value, p.PeriodSeconds))
			}
		}
		b.WriteString("\n")
	}

	renderLabelsAndAnnotations(&b, sectionStyle, hpa.Labels, hpa.Annotations)

	if len(hpa.Status.Conditions) > 0 {
		b.WriteString(sectionStyle.Render("Conditions") + "\n")
		for _, c := range hpa.Status.Conditions {
			b.WriteString(fmt.Sprintf("  %-25s %s  %s\n", c.Type, string(c.Status), c.Message))
		}
	}

	return b.String()
}

func getHPAYAML(ctx context.Context, client *Client, namespace, name string) ([]byte, error) {
	obj, err := client.GetHPA(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	obj.ManagedFields = nil
	return yaml.Marshal(obj)
}

func updateHPA(ctx context.Context, client *Client, namespace, _ string, data []byte) error {
	var obj autoscalingv2.HorizontalPodAutoscaler
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return client.UpdateHPA(ctx, namespace, &obj)
}

// ---------------------------------------------------------------------------
// Delete functions for tier-1 resources
// ---------------------------------------------------------------------------

func deleteStatefulSet(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteStatefulSet(ctx, namespace, name)
}

func deleteDaemonSet(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteDaemonSet(ctx, namespace, name)
}

func deleteReplicaSet(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteReplicaSet(ctx, namespace, name)
}

func deleteJob(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteJob(ctx, namespace, name)
}

func deleteCronJob(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteCronJob(ctx, namespace, name)
}

func deleteNode(ctx context.Context, client *Client, _, name string) error {
	return client.DeleteNode(ctx, name)
}

func deletePersistentVolume(ctx context.Context, client *Client, _, name string) error {
	return client.DeletePersistentVolume(ctx, name)
}

func deletePersistentVolumeClaim(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeletePersistentVolumeClaim(ctx, namespace, name)
}

func deleteServiceAccount(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteServiceAccount(ctx, namespace, name)
}

func deleteIngress(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteIngress(ctx, namespace, name)
}

func deleteHPA(ctx context.Context, client *Client, namespace, name string) error {
	return client.DeleteHPA(ctx, namespace, name)
}

// ---------------------------------------------------------------------------
// Shared describe helpers
// ---------------------------------------------------------------------------

// renderLabelsAndAnnotations writes Labels and Annotations sections.
func renderLabelsAndAnnotations(b *strings.Builder, sectionStyle lipgloss.Style, labels, annotations map[string]string) {
	if len(labels) > 0 {
		b.WriteString(sectionStyle.Render("Labels") + "\n")
		for k, v := range labels {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}

	if len(annotations) > 0 {
		b.WriteString(sectionStyle.Render("Annotations") + "\n")
		for k, v := range annotations {
			b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
		b.WriteString("\n")
	}
}

// renderPodTemplate writes the Pod Template section (containers, volumes, etc.)
func renderPodTemplate(b *strings.Builder, labelStyle, sectionStyle lipgloss.Style, tmpl v1.PodTemplateSpec) {
	b.WriteString(sectionStyle.Render("Pod Template") + "\n")

	if tmpl.Spec.ServiceAccountName != "" {
		b.WriteString(labelStyle.Render("  Service Account: ") + tmpl.Spec.ServiceAccountName + "\n")
	}
	if tmpl.Spec.NodeSelector != nil {
		selectors := make([]string, 0, len(tmpl.Spec.NodeSelector))
		for k, v := range tmpl.Spec.NodeSelector {
			selectors = append(selectors, k+"="+v)
		}
		b.WriteString(labelStyle.Render("  Node Selector:   ") + strings.Join(selectors, ", ") + "\n")
	}

	for _, c := range tmpl.Spec.Containers {
		b.WriteString(labelStyle.Render("  "+c.Name) + "\n")
		b.WriteString(fmt.Sprintf("    Image:   %s\n", c.Image))
		if len(c.Ports) > 0 {
			ports := make([]string, 0, len(c.Ports))
			for _, p := range c.Ports {
				ports = append(ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
			}
			b.WriteString(fmt.Sprintf("    Ports:   %s\n", strings.Join(ports, ", ")))
		}
		if len(c.Env) > 0 {
			b.WriteString("    Env:\n")
			for _, e := range c.Env {
				if e.ValueFrom != nil {
					src := "<ref>"
					if e.ValueFrom.SecretKeyRef != nil {
						src = fmt.Sprintf("secret:%s/%s", e.ValueFrom.SecretKeyRef.Name, e.ValueFrom.SecretKeyRef.Key)
					} else if e.ValueFrom.ConfigMapKeyRef != nil {
						src = fmt.Sprintf("configmap:%s/%s", e.ValueFrom.ConfigMapKeyRef.Name, e.ValueFrom.ConfigMapKeyRef.Key)
					} else if e.ValueFrom.FieldRef != nil {
						src = fmt.Sprintf("field:%s", e.ValueFrom.FieldRef.FieldPath)
					}
					b.WriteString(fmt.Sprintf("      %s = %s\n", e.Name, src))
				} else {
					b.WriteString(fmt.Sprintf("      %s = %s\n", e.Name, e.Value))
				}
			}
		}
		if len(c.VolumeMounts) > 0 {
			b.WriteString("    Mounts:\n")
			for _, vm := range c.VolumeMounts {
				ro := ""
				if vm.ReadOnly {
					ro = " (ro)"
				}
				b.WriteString(fmt.Sprintf("      %s -> %s%s\n", vm.Name, vm.MountPath, ro))
			}
		}
		if c.Resources.Requests != nil || c.Resources.Limits != nil {
			b.WriteString("    Resources:\n")
			if c.Resources.Requests != nil {
				b.WriteString("      Requests:\n")
				for res, qty := range c.Resources.Requests {
					b.WriteString(fmt.Sprintf("        %s: %s\n", res, qty.String()))
				}
			}
			if c.Resources.Limits != nil {
				b.WriteString("      Limits:\n")
				for res, qty := range c.Resources.Limits {
					b.WriteString(fmt.Sprintf("        %s: %s\n", res, qty.String()))
				}
			}
		}
	}

	if len(tmpl.Spec.InitContainers) > 0 {
		b.WriteString(sectionStyle.Render("  Init Containers") + "\n")
		for _, c := range tmpl.Spec.InitContainers {
			b.WriteString(labelStyle.Render("  "+c.Name) + "\n")
			b.WriteString(fmt.Sprintf("    Image:   %s\n", c.Image))
		}
	}

	if len(tmpl.Spec.Volumes) > 0 {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("  Volumes") + "\n")
		for _, vol := range tmpl.Spec.Volumes {
			b.WriteString(fmt.Sprintf("    %s", vol.Name))
			switch {
			case vol.ConfigMap != nil:
				b.WriteString(fmt.Sprintf(" (ConfigMap: %s)", vol.ConfigMap.Name))
			case vol.Secret != nil:
				b.WriteString(fmt.Sprintf(" (Secret: %s)", vol.Secret.SecretName))
			case vol.PersistentVolumeClaim != nil:
				b.WriteString(fmt.Sprintf(" (PVC: %s)", vol.PersistentVolumeClaim.ClaimName))
			case vol.EmptyDir != nil:
				b.WriteString(" (EmptyDir)")
			case vol.HostPath != nil:
				b.WriteString(fmt.Sprintf(" (HostPath: %s)", vol.HostPath.Path))
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
}

// ptrInt32Str returns the string of a *int32 or a default.
func ptrInt32Str(p *int32, defaultVal string) string {
	if p == nil {
		return defaultVal
	}
	return fmt.Sprintf("%d", *p)
}
