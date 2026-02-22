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
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	streamspdy "k8s.io/apimachinery/pkg/util/httpstream/spdy"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

// Client wraps a kubernetes.Interface for use by the TUI.
// No kubeconfig file is used — the clientset is created from
// in-memory TLS credentials issued by Teleport.
type Client struct {
	clientset  kubernetes.Interface
	dynamic    dynamic.Interface
	restConfig *rest.Config
	cluster    string
}

// NewClient creates a new Client with the given Kubernetes clientset, REST config, and cluster name.
func NewClient(clientset kubernetes.Interface, restConfig *rest.Config, cluster string) *Client {
	dynClient, _ := dynamic.NewForConfig(restConfig)
	return &Client{
		clientset:  clientset,
		dynamic:    dynClient,
		restConfig: restConfig,
		cluster:    cluster,
	}
}

// ListPods returns all pods in the given namespace. If namespace is empty,
// pods from all namespaces are returned.
func (c *Client) ListPods(ctx context.Context, namespace string) ([]v1.Pod, error) {
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetPodLogs returns a stream of logs for the given pod/container.
// If follow is true, the stream will remain open for new log lines.
func (c *Client) GetPodLogs(ctx context.Context, namespace, pod, container string, follow bool) (io.ReadCloser, error) {
	opts := &v1.PodLogOptions{
		Follow:    follow,
		Container: container,
	}
	return c.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
}

// DescribePod returns the full Pod object for inspection.
func (c *Client) DescribePod(ctx context.Context, namespace, pod string) (*v1.Pod, error) {
	return c.clientset.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
}

// ListNamespaces returns all namespaces the user has access to.
func (c *Client) ListNamespaces(ctx context.Context) ([]v1.Namespace, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// ListServices returns all services in the given namespace.
func (c *Client) ListServices(ctx context.Context, namespace string) ([]v1.Service, error) {
	list, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetService returns a single service by name.
func (c *Client) GetService(ctx context.Context, namespace, name string) (*v1.Service, error) {
	return c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListConfigMaps returns all configmaps in the given namespace.
func (c *Client) ListConfigMaps(ctx context.Context, namespace string) ([]v1.ConfigMap, error) {
	list, err := c.clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetConfigMap returns a single configmap by name.
func (c *Client) GetConfigMap(ctx context.Context, namespace, name string) (*v1.ConfigMap, error) {
	return c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListSecrets returns all secrets in the given namespace.
func (c *Client) ListSecrets(ctx context.Context, namespace string) ([]v1.Secret, error) {
	list, err := c.clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetSecret returns a single secret by name.
func (c *Client) GetSecret(ctx context.Context, namespace, name string) (*v1.Secret, error) {
	return c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// ListDeployments returns all deployments in the given namespace.
func (c *Client) ListDeployments(ctx context.Context, namespace string) ([]appsv1.Deployment, error) {
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetDeployment returns a single deployment by name.
func (c *Client) GetDeployment(ctx context.Context, namespace, name string) (*appsv1.Deployment, error) {
	return c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdatePod updates an existing pod.
func (c *Client) UpdatePod(ctx context.Context, namespace string, pod *v1.Pod) error {
	_, err := c.clientset.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

// UpdateService updates an existing service.
func (c *Client) UpdateService(ctx context.Context, namespace string, svc *v1.Service) error {
	_, err := c.clientset.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	return err
}

// UpdateConfigMap updates an existing configmap.
func (c *Client) UpdateConfigMap(ctx context.Context, namespace string, cm *v1.ConfigMap) error {
	_, err := c.clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

// UpdateSecret updates an existing secret.
func (c *Client) UpdateSecret(ctx context.Context, namespace string, secret *v1.Secret) error {
	_, err := c.clientset.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	return err
}

// UpdateDeployment updates an existing deployment.
func (c *Client) UpdateDeployment(ctx context.Context, namespace string, dep *appsv1.Deployment) error {
	_, err := c.clientset.AppsV1().Deployments(namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

// ---------------------------------------------------------------------------
// Delete methods for base resources
// ---------------------------------------------------------------------------

// DeletePod deletes a pod by name.
func (c *Client) DeletePod(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteService deletes a service by name.
func (c *Client) DeleteService(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteConfigMap deletes a configmap by name.
func (c *Client) DeleteConfigMap(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteSecret deletes a secret by name.
func (c *Client) DeleteSecret(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DeleteDeployment deletes a deployment by name.
func (c *Client) DeleteDeployment(ctx context.Context, namespace, name string) error {
	return c.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// Dynamic client methods
// ---------------------------------------------------------------------------

// DynamicList lists resources using the dynamic client.
// For cluster-scoped resources, pass namespace as "".
func (c *Client) DynamicList(ctx context.Context, gvr schema.GroupVersionResource, namespace string) (*unstructured.UnstructuredList, error) {
	if namespace == "" {
		return c.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
	}
	return c.dynamic.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
}

// DynamicGet gets a single resource using the dynamic client.
func (c *Client) DynamicGet(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) (*unstructured.Unstructured, error) {
	if namespace == "" {
		return c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	return c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
}

// DynamicDelete deletes a resource using the dynamic client.
func (c *Client) DynamicDelete(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) error {
	if namespace == "" {
		return c.dynamic.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	}
	return c.dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// DynamicUpdate updates a resource using the dynamic client.
func (c *Client) DynamicUpdate(ctx context.Context, gvr schema.GroupVersionResource, namespace string, obj *unstructured.Unstructured) error {
	if namespace == "" {
		_, err := c.dynamic.Resource(gvr).Update(ctx, obj, metav1.UpdateOptions{})
		return err
	}
	_, err := c.dynamic.Resource(gvr).Namespace(namespace).Update(ctx, obj, metav1.UpdateOptions{})
	return err
}

// DiscoverAllResources returns all API resources the server supports.
// It returns partial results even if some API groups fail.
func (c *Client) DiscoverAllResources(ctx context.Context) ([]*metav1.APIResourceList, error) {
	return c.clientset.Discovery().ServerPreferredResources()
}

// ---------------------------------------------------------------------------
// StatefulSets
// ---------------------------------------------------------------------------

// ListStatefulSets returns all statefulsets in the given namespace.
func (c *Client) ListStatefulSets(ctx context.Context, namespace string) ([]appsv1.StatefulSet, error) {
	list, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetStatefulSet returns a single statefulset by name.
func (c *Client) GetStatefulSet(ctx context.Context, namespace, name string) (*appsv1.StatefulSet, error) {
	return c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateStatefulSet updates an existing statefulset.
func (c *Client) UpdateStatefulSet(ctx context.Context, namespace string, sts *appsv1.StatefulSet) error {
	_, err := c.clientset.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}

// DeleteStatefulSet deletes a statefulset by name.
func (c *Client) DeleteStatefulSet(ctx context.Context, namespace, name string) error {
	return c.clientset.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// DaemonSets
// ---------------------------------------------------------------------------

// ListDaemonSets returns all daemonsets in the given namespace.
func (c *Client) ListDaemonSets(ctx context.Context, namespace string) ([]appsv1.DaemonSet, error) {
	list, err := c.clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetDaemonSet returns a single daemonset by name.
func (c *Client) GetDaemonSet(ctx context.Context, namespace, name string) (*appsv1.DaemonSet, error) {
	return c.clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateDaemonSet updates an existing daemonset.
func (c *Client) UpdateDaemonSet(ctx context.Context, namespace string, ds *appsv1.DaemonSet) error {
	_, err := c.clientset.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{})
	return err
}

// DeleteDaemonSet deletes a daemonset by name.
func (c *Client) DeleteDaemonSet(ctx context.Context, namespace, name string) error {
	return c.clientset.AppsV1().DaemonSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// ReplicaSets
// ---------------------------------------------------------------------------

// ListReplicaSets returns all replicasets in the given namespace.
func (c *Client) ListReplicaSets(ctx context.Context, namespace string) ([]appsv1.ReplicaSet, error) {
	list, err := c.clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetReplicaSet returns a single replicaset by name.
func (c *Client) GetReplicaSet(ctx context.Context, namespace, name string) (*appsv1.ReplicaSet, error) {
	return c.clientset.AppsV1().ReplicaSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateReplicaSet updates an existing replicaset.
func (c *Client) UpdateReplicaSet(ctx context.Context, namespace string, rs *appsv1.ReplicaSet) error {
	_, err := c.clientset.AppsV1().ReplicaSets(namespace).Update(ctx, rs, metav1.UpdateOptions{})
	return err
}

// DeleteReplicaSet deletes a replicaset by name.
func (c *Client) DeleteReplicaSet(ctx context.Context, namespace, name string) error {
	return c.clientset.AppsV1().ReplicaSets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

// ListJobs returns all jobs in the given namespace.
func (c *Client) ListJobs(ctx context.Context, namespace string) ([]batchv1.Job, error) {
	list, err := c.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetJob returns a single job by name.
func (c *Client) GetJob(ctx context.Context, namespace, name string) (*batchv1.Job, error) {
	return c.clientset.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateJob updates an existing job.
func (c *Client) UpdateJob(ctx context.Context, namespace string, job *batchv1.Job) error {
	_, err := c.clientset.BatchV1().Jobs(namespace).Update(ctx, job, metav1.UpdateOptions{})
	return err
}

// DeleteJob deletes a job by name.
func (c *Client) DeleteJob(ctx context.Context, namespace, name string) error {
	return c.clientset.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// CronJobs
// ---------------------------------------------------------------------------

// ListCronJobs returns all cronjobs in the given namespace.
func (c *Client) ListCronJobs(ctx context.Context, namespace string) ([]batchv1.CronJob, error) {
	list, err := c.clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetCronJob returns a single cronjob by name.
func (c *Client) GetCronJob(ctx context.Context, namespace, name string) (*batchv1.CronJob, error) {
	return c.clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateCronJob updates an existing cronjob.
func (c *Client) UpdateCronJob(ctx context.Context, namespace string, cj *batchv1.CronJob) error {
	_, err := c.clientset.BatchV1().CronJobs(namespace).Update(ctx, cj, metav1.UpdateOptions{})
	return err
}

// DeleteCronJob deletes a cronjob by name.
func (c *Client) DeleteCronJob(ctx context.Context, namespace, name string) error {
	return c.clientset.BatchV1().CronJobs(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// Nodes (cluster-scoped)
// ---------------------------------------------------------------------------

// ListNodes returns all nodes.
func (c *Client) ListNodes(ctx context.Context) ([]v1.Node, error) {
	list, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetNode returns a single node by name.
func (c *Client) GetNode(ctx context.Context, name string) (*v1.Node, error) {
	return c.clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
}

// UpdateNode updates an existing node.
func (c *Client) UpdateNode(ctx context.Context, node *v1.Node) error {
	_, err := c.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	return err
}

// DeleteNode deletes a node by name.
func (c *Client) DeleteNode(ctx context.Context, name string) error {
	return c.clientset.CoreV1().Nodes().Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// PersistentVolumes (cluster-scoped)
// ---------------------------------------------------------------------------

// ListPersistentVolumes returns all persistent volumes.
func (c *Client) ListPersistentVolumes(ctx context.Context) ([]v1.PersistentVolume, error) {
	list, err := c.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetPersistentVolume returns a single persistent volume by name.
func (c *Client) GetPersistentVolume(ctx context.Context, name string) (*v1.PersistentVolume, error) {
	return c.clientset.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
}

// UpdatePersistentVolume updates an existing persistent volume.
func (c *Client) UpdatePersistentVolume(ctx context.Context, pv *v1.PersistentVolume) error {
	_, err := c.clientset.CoreV1().PersistentVolumes().Update(ctx, pv, metav1.UpdateOptions{})
	return err
}

// DeletePersistentVolume deletes a persistent volume by name.
func (c *Client) DeletePersistentVolume(ctx context.Context, name string) error {
	return c.clientset.CoreV1().PersistentVolumes().Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// PersistentVolumeClaims
// ---------------------------------------------------------------------------

// ListPersistentVolumeClaims returns all PVCs in the given namespace.
func (c *Client) ListPersistentVolumeClaims(ctx context.Context, namespace string) ([]v1.PersistentVolumeClaim, error) {
	list, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetPersistentVolumeClaim returns a single PVC by name.
func (c *Client) GetPersistentVolumeClaim(ctx context.Context, namespace, name string) (*v1.PersistentVolumeClaim, error) {
	return c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdatePersistentVolumeClaim updates an existing PVC.
func (c *Client) UpdatePersistentVolumeClaim(ctx context.Context, namespace string, pvc *v1.PersistentVolumeClaim) error {
	_, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{})
	return err
}

// DeletePersistentVolumeClaim deletes a PVC by name.
func (c *Client) DeletePersistentVolumeClaim(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// ServiceAccounts
// ---------------------------------------------------------------------------

// ListServiceAccounts returns all service accounts in the given namespace.
func (c *Client) ListServiceAccounts(ctx context.Context, namespace string) ([]v1.ServiceAccount, error) {
	list, err := c.clientset.CoreV1().ServiceAccounts(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetServiceAccount returns a single service account by name.
func (c *Client) GetServiceAccount(ctx context.Context, namespace, name string) (*v1.ServiceAccount, error) {
	return c.clientset.CoreV1().ServiceAccounts(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateServiceAccount updates an existing service account.
func (c *Client) UpdateServiceAccount(ctx context.Context, namespace string, sa *v1.ServiceAccount) error {
	_, err := c.clientset.CoreV1().ServiceAccounts(namespace).Update(ctx, sa, metav1.UpdateOptions{})
	return err
}

// DeleteServiceAccount deletes a service account by name.
func (c *Client) DeleteServiceAccount(ctx context.Context, namespace, name string) error {
	return c.clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// Ingresses
// ---------------------------------------------------------------------------

// ListIngresses returns all ingresses in the given namespace.
func (c *Client) ListIngresses(ctx context.Context, namespace string) ([]networkingv1.Ingress, error) {
	list, err := c.clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetIngress returns a single ingress by name.
func (c *Client) GetIngress(ctx context.Context, namespace, name string) (*networkingv1.Ingress, error) {
	return c.clientset.NetworkingV1().Ingresses(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateIngress updates an existing ingress.
func (c *Client) UpdateIngress(ctx context.Context, namespace string, ing *networkingv1.Ingress) error {
	_, err := c.clientset.NetworkingV1().Ingresses(namespace).Update(ctx, ing, metav1.UpdateOptions{})
	return err
}

// DeleteIngress deletes an ingress by name.
func (c *Client) DeleteIngress(ctx context.Context, namespace, name string) error {
	return c.clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// HorizontalPodAutoscalers
// ---------------------------------------------------------------------------

// ListHPAs returns all HPAs in the given namespace.
func (c *Client) ListHPAs(ctx context.Context, namespace string) ([]autoscalingv2.HorizontalPodAutoscaler, error) {
	list, err := c.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

// GetHPA returns a single HPA by name.
func (c *Client) GetHPA(ctx context.Context, namespace, name string) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	return c.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, name, metav1.GetOptions{})
}

// UpdateHPA updates an existing HPA.
func (c *Client) UpdateHPA(ctx context.Context, namespace string, hpa *autoscalingv2.HorizontalPodAutoscaler) error {
	_, err := c.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Update(ctx, hpa, metav1.UpdateOptions{})
	return err
}

// DeleteHPA deletes an HPA by name.
func (c *Client) DeleteHPA(ctx context.Context, namespace, name string) error {
	return c.clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

// ---------------------------------------------------------------------------
// Port Forwarding
// ---------------------------------------------------------------------------

// PortForwardResult holds the result of a successful port-forward.
type PortForwardResult struct {
	LocalPort uint16
	StopCh    chan struct{}
}

// PortForward starts forwarding a local port to the given remote port on a pod.
// If localPort is 0, the OS assigns a free port automatically.
func (c *Client) PortForward(namespace, pod string, localPort, remotePort uint16) (*PortForwardResult, error) {
	u, err := url.Parse(c.restConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("parsing host URL: %w", err)
	}
	u.Scheme = "https"
	u.Path = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", namespace, pod)

	tlsConfig, err := rest.TLSConfigFor(c.restConfig)
	if err != nil {
		return nil, fmt.Errorf("building TLS config: %w", err)
	}

	upgradeRoundTripper, err := streamspdy.NewRoundTripper(tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("creating SPDY round tripper: %w", err)
	}
	dialer := spdy.NewDialer(upgradeRoundTripper, &http.Client{Transport: upgradeRoundTripper}, "POST", u)

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	fwd, err := portforward.New(dialer, ports, stopCh, readyCh, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("creating port forwarder: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- fwd.ForwardPorts()
	}()

	select {
	case <-readyCh:
	case err := <-errCh:
		return nil, fmt.Errorf("port forwarding failed: %w", err)
	}

	forwardedPorts, err := fwd.GetPorts()
	if err != nil {
		close(stopCh)
		return nil, fmt.Errorf("getting forwarded ports: %w", err)
	}
	if len(forwardedPorts) == 0 {
		close(stopCh)
		return nil, fmt.Errorf("no ports forwarded")
	}

	return &PortForwardResult{
		LocalPort: forwardedPorts[0].Local,
		StopCh:    stopCh,
	}, nil
}

// ExecConfig holds parameters for an exec session.
type ExecConfig struct {
	Namespace string
	Pod       string
	Container string
	Stdin     io.Reader
	Stdout    io.Writer
	SizeQueue remotecommand.TerminalSizeQueue
}

// ExecPod opens an interactive shell in a pod via WebSocket exec.
func (c *Client) ExecPod(ctx context.Context, cfg ExecConfig) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(cfg.Pod).
		Namespace(cfg.Namespace).SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: cfg.Container,
		Command:   []string{"/bin/sh"},
		Stdin:     true,
		Stdout:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	wsExec, err := remotecommand.NewWebSocketExecutor(c.restConfig, "POST", req.URL().String())
	if err != nil {
		return err
	}

	return wsExec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             cfg.Stdin,
		Stdout:            cfg.Stdout,
		Tty:               true,
		TerminalSizeQueue: cfg.SizeQueue,
	})
}

// ---------------------------------------------------------------------------
// Non-TTY exec (for file copy)
// ---------------------------------------------------------------------------

// ExecCommandConfig holds parameters for a non-interactive exec.
type ExecCommandConfig struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
}

// ExecCommand runs a command in a container without a TTY.
func (c *Client) ExecCommand(ctx context.Context, cfg ExecCommandConfig) error {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(cfg.Pod).
		Namespace(cfg.Namespace).SubResource("exec")
	req.VersionedParams(&v1.PodExecOptions{
		Container: cfg.Container,
		Command:   cfg.Command,
		Stdin:     cfg.Stdin != nil,
		Stdout:    cfg.Stdout != nil,
		Stderr:    cfg.Stderr != nil,
		TTY:       false,
	}, scheme.ParameterCodec)

	wsExec, err := remotecommand.NewWebSocketExecutor(c.restConfig, "POST", req.URL().String())
	if err != nil {
		return err
	}

	return wsExec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  cfg.Stdin,
		Stdout: cfg.Stdout,
		Stderr: cfg.Stderr,
	})
}

// ---------------------------------------------------------------------------
// File copy (via exec + tar)
// ---------------------------------------------------------------------------

// CopyFromPod downloads a file or directory from a container to the local filesystem.
// remotePath is the absolute path inside the container; localPath is the local destination.
func (c *Client) CopyFromPod(ctx context.Context, namespace, pod, container, remotePath, localPath string) error {
	dir := path.Dir(remotePath)
	base := path.Base(remotePath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := c.ExecCommand(ctx, ExecCommandConfig{
		Namespace: namespace,
		Pod:       pod,
		Container: container,
		Command:   []string{"tar", "cf", "-", "-C", dir, base},
		Stdout:    &stdout,
		Stderr:    &stderr,
	})
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return fmt.Errorf("%w: %s", err, errMsg)
		}
		return err
	}

	return extractTar(&stdout, localPath)
}

// CopyToPod uploads a local file or directory into a container.
// localPath is the local source; remotePath is the destination directory inside the container.
func (c *Client) CopyToPod(ctx context.Context, namespace, pod, container, localPath, remotePath string) error {
	var tarBuf bytes.Buffer
	if err := createTar(localPath, &tarBuf); err != nil {
		return fmt.Errorf("creating tar: %w", err)
	}

	var stderr bytes.Buffer
	err := c.ExecCommand(ctx, ExecCommandConfig{
		Namespace: namespace,
		Pod:       pod,
		Container: container,
		Command:   []string{"tar", "xf", "-", "-C", remotePath},
		Stdin:     &tarBuf,
		Stderr:    &stderr,
	})
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg != "" {
			return fmt.Errorf("%w: %s", err, errMsg)
		}
		return err
	}
	return nil
}

// extractTar reads a tar stream and extracts it to destDir.
func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		target := filepath.Join(destDir, filepath.Clean(hdr.Name))

		// Prevent path traversal.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			return fmt.Errorf("tar entry %q escapes destination directory", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
}

// createTar creates a tar archive from localPath and writes it to w.
func createTar(localPath string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	info, err := os.Stat(localPath)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		// Single file.
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = info.Name()
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	}

	// Directory: walk and add all files.
	base := filepath.Base(localPath)
	return filepath.Walk(localPath, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(localPath, p)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(filepath.Join(base, rel))
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}
