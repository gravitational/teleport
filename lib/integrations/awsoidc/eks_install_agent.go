/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package awsoidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	appsv1ac "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1ac "k8s.io/client-go/applyconfigurations/core/v1"
	metav1ac "k8s.io/client-go/applyconfigurations/meta/v1"
	policyv1ac "k8s.io/client-go/applyconfigurations/policy/v1"
	rbacv1ac "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// agentFieldManager is the server-side-apply field manager used for all
// resources created by the EKS kube-agent installer. It is distinct from
// Helm's "helm" field manager so a future helm upgrade of the same agent
// will take ownership of conflicting fields rather than silently lose them.
const agentFieldManager = "teleport-kube-agent-installer"

// kubeAgentUpdaterImage is the default updater container image. Matches the
// teleport-kube-updater chart `image` default.
const kubeAgentUpdaterImage = "public.ecr.aws/gravitational/teleport-kube-agent-updater"

// teleportAgentImageOSS and teleportAgentImageEnterprise match the
// teleport-kube-agent chart `image` / `enterpriseImage` defaults.
const (
	teleportAgentImageOSS        = "public.ecr.aws/gravitational/teleport-distroless"
	teleportAgentImageEnterprise = "public.ecr.aws/gravitational/teleport-ent-distroless"
)

// installKubeAgentParams bundles the inputs required to install the
// teleport-kube-agent into an EKS cluster.
type installKubeAgentParams struct {
	eksCluster   *eksTypes.Cluster
	proxyAddr    string
	joinToken    string
	resourceID   string
	clientGetter genericclioptions.RESTClientGetter
	log          *slog.Logger
	req          EnrollEKSClustersRequest
}

// installKubeAgent applies the Kubernetes resources that the
// teleport-kube-agent Helm chart would produce for the inputs supplied by
// the EKS enrollment flow. It uses server-side apply so repeated invocations
// are idempotent.
func installKubeAgent(ctx context.Context, p installKubeAgentParams) error {
	restCfg, err := p.clientGetter.ToRESTConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return trace.Wrap(err)
	}

	roles := "kube"
	// todo(anton): Remove check for 13 once Teleport cloud is unblocked to move from v13 chart.
	if p.req.EnableAppDiscovery && !strings.HasPrefix(p.req.AgentVersion, "13") {
		roles = "kube,app,discovery"
	}

	enterprise := modules.GetModules().BuildType() == modules.BuildEnterprise
	useUpdater := p.req.IsCloud && p.req.EnableAutoUpgrades

	replicaCount := int32(1)
	enablePDB := false
	if useUpdater {
		replicaCount = 2
		enablePDB = true
	}

	eksTags := make(map[string]string, len(p.eksCluster.Tags))
	maps.Copy(eksTags, p.eksCluster.Tags)
	eksTags[types.OriginLabel] = types.OriginCloud
	kubeCluster, err := common.NewKubeClusterFromAWSEKS(aws.ToString(p.eksCluster.Name), aws.ToString(p.eksCluster.Arn), eksTags)
	if err != nil {
		return trace.Wrap(err)
	}
	common.ApplyEKSNameSuffix(kubeCluster)

	kubeClusterName := kubeCluster.GetName()
	agentLabels := kubeAgentLabels(kubeCluster, p.resourceID, p.req.ExtraLabels)

	configYAML, err := renderTeleportConfig(p.proxyAddr, roles, kubeClusterName, agentLabels)
	if err != nil {
		return trace.Wrap(err)
	}
	configSum := sha256.Sum256(configYAML)
	configChecksum := hex.EncodeToString(configSum[:])

	image := kubeAgentImage(enterprise, p.req.AgentVersion)
	opts := metav1.ApplyOptions{FieldManager: agentFieldManager, Force: true}

	if err := applyNamespace(ctx, clientset, opts); err != nil {
		return trace.Wrap(err, "applying namespace")
	}
	if err := applyServiceAccount(ctx, clientset, opts); err != nil {
		return trace.Wrap(err, "applying service account")
	}
	if err := applyAgentClusterRole(ctx, clientset, roles, opts); err != nil {
		return trace.Wrap(err, "applying cluster role")
	}
	if err := applyAgentClusterRoleBinding(ctx, clientset, opts); err != nil {
		return trace.Wrap(err, "applying cluster role binding")
	}
	if err := applyAgentRole(ctx, clientset, opts); err != nil {
		return trace.Wrap(err, "applying role")
	}
	if err := applyAgentRoleBinding(ctx, clientset, opts); err != nil {
		return trace.Wrap(err, "applying role binding")
	}
	if err := applyJoinTokenSecret(ctx, clientset, p.joinToken, opts); err != nil {
		return trace.Wrap(err, "applying join-token secret")
	}
	if err := applyConfigMap(ctx, clientset, configYAML, opts); err != nil {
		return trace.Wrap(err, "applying config map")
	}
	if enablePDB {
		if err := applyPodDisruptionBudget(ctx, clientset, opts); err != nil {
			return trace.Wrap(err, "applying pod disruption budget")
		}
	}
	if err := applyStatefulSet(ctx, clientset, statefulSetParams{
		image:           image,
		agentVersion:    p.req.AgentVersion,
		replicaCount:    replicaCount,
		configChecksum:  configChecksum,
		useUpdater:      useUpdater,
		antiAffinityOn:  replicaCount > 1,
		topologySpread:  true,
	}, opts); err != nil {
		return trace.Wrap(err, "applying stateful set")
	}

	if useUpdater {
		if err := applyUpdaterServiceAccount(ctx, clientset, opts); err != nil {
			return trace.Wrap(err, "applying updater service account")
		}
		if err := applyUpdaterRole(ctx, clientset, opts); err != nil {
			return trace.Wrap(err, "applying updater role")
		}
		if err := applyUpdaterRoleBinding(ctx, clientset, opts); err != nil {
			return trace.Wrap(err, "applying updater role binding")
		}
		if err := applyUpdaterDeployment(ctx, clientset, updaterDeploymentParams{
			proxyAddr:      p.proxyAddr,
			agentVersion:   p.req.AgentVersion,
			releaseChannel: "stable/cloud",
			baseImage:      kubeAgentBaseImage(enterprise),
		}, opts); err != nil {
			return trace.Wrap(err, "applying updater deployment")
		}
	}

	return nil
}

// checkAgentAlreadyInstalled reports whether the teleport-kube-agent is
// already deployed on the cluster. It detects both agents installed by this
// code path (by looking for the StatefulSet) and agents installed by the
// prior Helm-based path (by looking for a helm release storage Secret). The
// six-attempt backoff loop mirrors the previous implementation, giving EKS
// access-entry authorization time to propagate.
func checkAgentAlreadyInstalled(ctx context.Context, clientGetter genericclioptions.RESTClientGetter) (bool, error) {
	restCfg, err := clientGetter.ToRESTConfig()
	if err != nil {
		return false, trace.Wrap(err)
	}
	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return false, trace.Wrap(err)
	}

	var lastErr error
	for attempt := 1; attempt <= 6; attempt++ {
		installed, err := checkAgentInstalledOnce(ctx, clientset)
		if err == nil {
			return installed, nil
		}
		lastErr = err

		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return false, trace.NewAggregate(err, ctx.Err())
		}
	}
	return false, trace.Wrap(lastErr)
}

func checkAgentInstalledOnce(ctx context.Context, clientset kubernetes.Interface) (bool, error) {
	// Detect legacy helm-installed agents by looking for the release storage
	// secret helm writes when a chart is installed.
	secrets, err := clientset.CoreV1().Secrets(agentNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "owner=helm,name=" + agentName,
	})
	switch {
	case err == nil && len(secrets.Items) > 0:
		return true, nil
	case err != nil && !kubeerrors.IsNotFound(err):
		return false, trace.Wrap(err)
	}

	// Detect agents installed by this code path.
	_, err = clientset.AppsV1().StatefulSets(agentNamespace).Get(ctx, agentName, metav1.GetOptions{})
	switch {
	case err == nil:
		return true, nil
	case kubeerrors.IsNotFound(err):
		return false, nil
	default:
		return false, trace.Wrap(err)
	}
}

func kubeAgentBaseImage(enterprise bool) string {
	if enterprise {
		return teleportAgentImageEnterprise
	}
	return teleportAgentImageOSS
}

func kubeAgentImage(enterprise bool, version string) string {
	return kubeAgentBaseImage(enterprise) + ":" + version
}

// renderTeleportConfig builds the teleport.yaml contents that the
// teleport-kube-agent.config template would produce for the supplied values.
func renderTeleportConfig(proxyAddr, roles, kubeClusterName string, labels map[string]any) ([]byte, error) {
	appRoleEnabled := strings.Contains(roles, "app")
	discoveryEnabled := strings.Contains(roles, "discovery")
	appDiscoveryEnabled := appRoleEnabled && discoveryEnabled

	teleport := map[string]any{
		"join_params": map[string]any{
			"method":     "token",
			"token_name": "/etc/teleport-secrets/auth-token",
		},
		"proxy_server": proxyAddr,
		"log": map[string]any{
			"severity": "INFO",
			"output":   "stderr",
			"format": map[string]any{
				"output":       "text",
				"extra_fields": []string{"timestamp", "level", "component", "caller"},
			},
		},
	}

	kubeSvc := map[string]any{
		"enabled":           true,
		"kube_cluster_name": kubeClusterName,
	}
	if len(labels) > 0 {
		kubeSvc["labels"] = labels
	}

	appSvc := map[string]any{"enabled": false}
	if appRoleEnabled {
		resources := []any{}
		if appDiscoveryEnabled {
			resources = append(resources, map[string]any{
				"labels": map[string]any{
					"teleport.dev/kubernetes-cluster": kubeClusterName,
					"teleport.dev/origin":             "discovery-kubernetes",
				},
			})
		}
		appSvc = map[string]any{
			"enabled":   true,
			"resources": resources,
		}
	}

	discoverySvc := map[string]any{"enabled": false}
	if discoveryEnabled {
		discoverySvc = map[string]any{
			"enabled":         true,
			"discovery_group": kubeClusterName,
			"kubernetes": []any{
				map[string]any{
					"types":      []string{"app"},
					"namespaces": []string{"*"},
					"labels":     map[string]any{"*": "*"},
				},
			},
		}
	}

	cfg := map[string]any{
		"version":           "v3",
		"teleport":          teleport,
		"kubernetes_service": kubeSvc,
		"app_service":        appSvc,
		"db_service":         map[string]any{"enabled": false},
		"discovery_service":  discoverySvc,
		"jamf_service":       map[string]any{"enabled": false},
		"auth_service":       map[string]any{"enabled": false},
		"ssh_service":        map[string]any{"enabled": false},
		"proxy_service":      map[string]any{"enabled": false},
	}

	out, err := yaml.Marshal(cfg)
	return out, trace.Wrap(err)
}

func applyNamespace(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	ns := corev1ac.Namespace(agentNamespace)
	_, err := clientset.CoreV1().Namespaces().Apply(ctx, ns, opts)
	return trace.Wrap(err)
}

func applyServiceAccount(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	sa := corev1ac.ServiceAccount(agentName, agentNamespace).
		WithAutomountServiceAccountToken(false)
	_, err := clientset.CoreV1().ServiceAccounts(agentNamespace).Apply(ctx, sa, opts)
	return trace.Wrap(err)
}

func applyAgentClusterRole(ctx context.Context, clientset *kubernetes.Clientset, roles string, opts metav1.ApplyOptions) error {
	rules := []*rbacv1ac.PolicyRuleApplyConfiguration{
		rbacv1ac.PolicyRule().
			WithAPIGroups("").
			WithResources("users", "groups", "serviceaccounts").
			WithVerbs("impersonate"),
	}
	if strings.Contains(roles, "discovery") {
		rules = append(rules, rbacv1ac.PolicyRule().
			WithAPIGroups("").
			WithResources("services").
			WithVerbs("list"))
	}
	rules = append(rules,
		rbacv1ac.PolicyRule().
			WithAPIGroups("").
			WithResources("pods").
			WithVerbs("get"),
		rbacv1ac.PolicyRule().
			WithAPIGroups("authorization.k8s.io").
			WithResources("selfsubjectaccessreviews").
			WithVerbs("create"),
	)

	cr := rbacv1ac.ClusterRole(agentName).WithRules(rules...)
	_, err := clientset.RbacV1().ClusterRoles().Apply(ctx, cr, opts)
	return trace.Wrap(err)
}

func applyAgentClusterRoleBinding(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	crb := rbacv1ac.ClusterRoleBinding(agentName).
		WithRoleRef(rbacv1ac.RoleRef().
			WithAPIGroup("rbac.authorization.k8s.io").
			WithKind("ClusterRole").
			WithName(agentName)).
		WithSubjects(rbacv1ac.Subject().
			WithKind("ServiceAccount").
			WithName(agentName).
			WithNamespace(agentNamespace))
	_, err := clientset.RbacV1().ClusterRoleBindings().Apply(ctx, crb, opts)
	return trace.Wrap(err)
}

func applyAgentRole(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	r := rbacv1ac.Role(agentName, agentNamespace).
		WithRules(rbacv1ac.PolicyRule().
			WithAPIGroups("").
			WithResources("secrets").
			WithVerbs("create", "get", "update", "patch"))
	_, err := clientset.RbacV1().Roles(agentNamespace).Apply(ctx, r, opts)
	return trace.Wrap(err)
}

func applyAgentRoleBinding(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	rb := rbacv1ac.RoleBinding(agentName, agentNamespace).
		WithRoleRef(rbacv1ac.RoleRef().
			WithAPIGroup("rbac.authorization.k8s.io").
			WithKind("Role").
			WithName(agentName)).
		WithSubjects(rbacv1ac.Subject().
			WithKind("ServiceAccount").
			WithName(agentName).
			WithNamespace(agentNamespace))
	_, err := clientset.RbacV1().RoleBindings(agentNamespace).Apply(ctx, rb, opts)
	return trace.Wrap(err)
}

func applyJoinTokenSecret(ctx context.Context, clientset *kubernetes.Clientset, joinToken string, opts metav1.ApplyOptions) error {
	s := corev1ac.Secret(joinTokenSecretName, agentNamespace).
		WithType(corev1.SecretTypeOpaque).
		WithStringData(map[string]string{"auth-token": joinToken + "\n"})
	_, err := clientset.CoreV1().Secrets(agentNamespace).Apply(ctx, s, opts)
	return trace.Wrap(err)
}

func applyConfigMap(ctx context.Context, clientset *kubernetes.Clientset, configYAML []byte, opts metav1.ApplyOptions) error {
	cm := corev1ac.ConfigMap(agentName, agentNamespace).
		WithData(map[string]string{"teleport.yaml": string(configYAML)})
	_, err := clientset.CoreV1().ConfigMaps(agentNamespace).Apply(ctx, cm, opts)
	return trace.Wrap(err)
}

func applyPodDisruptionBudget(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	minAvailable := intstr.FromInt32(1)
	pdb := policyv1ac.PodDisruptionBudget(agentName, agentNamespace).
		WithLabels(map[string]string{"app": agentName}).
		WithSpec(policyv1ac.PodDisruptionBudgetSpec().
			WithMinAvailable(minAvailable).
			WithSelector(metav1ac.LabelSelector().
				WithMatchLabels(map[string]string{"app": agentName})))
	_, err := clientset.PolicyV1().PodDisruptionBudgets(agentNamespace).Apply(ctx, pdb, opts)
	return trace.Wrap(err)
}

// joinTokenSecretName matches the chart's joinTokenSecret.name default.
const joinTokenSecretName = "teleport-kube-agent-join-token"

type statefulSetParams struct {
	image          string
	agentVersion   string
	replicaCount   int32
	configChecksum string
	useUpdater     bool
	antiAffinityOn bool
	topologySpread bool
}

func applyStatefulSet(ctx context.Context, clientset *kubernetes.Clientset, p statefulSetParams, opts metav1.ApplyOptions) error {
	appSelector := map[string]string{"app": agentName}

	env := []*corev1ac.EnvVarApplyConfiguration{
		corev1ac.EnvVar().WithName("TELEPORT_INSTALL_METHOD_HELM_KUBE_AGENT").WithValue("true"),
		corev1ac.EnvVar().WithName("TELEPORT_REPLICA_NAME").
			WithValueFrom(corev1ac.EnvVarSource().
				WithFieldRef(corev1ac.ObjectFieldSelector().WithFieldPath("metadata.name"))),
		corev1ac.EnvVar().WithName("KUBE_NAMESPACE").
			WithValueFrom(corev1ac.EnvVarSource().
				WithFieldRef(corev1ac.ObjectFieldSelector().WithFieldPath("metadata.namespace"))),
		corev1ac.EnvVar().WithName("RELEASE_NAME").WithValue(agentName),
	}
	if p.useUpdater {
		env = append(env,
			corev1ac.EnvVar().WithName("TELEPORT_EXT_UPGRADER").WithValue("kube"),
			corev1ac.EnvVar().WithName("TELEPORT_EXT_UPGRADER_VERSION").WithValue(p.agentVersion),
		)
	}
	env = append(env, corev1ac.EnvVar().WithName("TELEPORT_KUBE_CLUSTER_DOMAIN").WithValue("cluster.local"))
	if p.useUpdater {
		env = append(env, corev1ac.EnvVar().WithName("TELEPORT_UPDATE_CONFIG_FILE").WithValue("/etc/updater-config/update.yaml"))
	}

	volumeMounts := []*corev1ac.VolumeMountApplyConfiguration{
		corev1ac.VolumeMount().WithMountPath("/etc/teleport").WithName("config").WithReadOnly(true),
		corev1ac.VolumeMount().WithMountPath("/etc/teleport-secrets").WithName("auth-token").WithReadOnly(true),
		corev1ac.VolumeMount().WithMountPath("/var/lib/teleport").WithName("data"),
	}
	if p.useUpdater {
		volumeMounts = append(volumeMounts,
			corev1ac.VolumeMount().WithMountPath("/etc/updater-config").WithName("updater-config").WithReadOnly(true))
	}

	volumes := []*corev1ac.VolumeApplyConfiguration{
		corev1ac.Volume().WithName("config").
			WithConfigMap(corev1ac.ConfigMapVolumeSource().WithName(agentName)),
		corev1ac.Volume().WithName("auth-token").
			WithSecret(corev1ac.SecretVolumeSource().WithSecretName(joinTokenSecretName)),
		corev1ac.Volume().WithName("data").
			WithEmptyDir(corev1ac.EmptyDirVolumeSource()),
	}
	if p.useUpdater {
		volumes = append(volumes,
			corev1ac.Volume().WithName("updater-config").
				WithConfigMap(corev1ac.ConfigMapVolumeSource().WithName(agentName+"-updater")))
	}

	container := corev1ac.Container().
		WithName("teleport").
		WithImage(p.image).
		WithImagePullPolicy(corev1.PullIfNotPresent).
		WithEnv(env...).
		WithArgs("--diag-addr=0.0.0.0:3000").
		WithSecurityContext(restrictedSecurityContext()).
		WithPorts(corev1ac.ContainerPort().WithName("diag").WithContainerPort(3000).WithProtocol(corev1.ProtocolTCP)).
		WithLivenessProbe(httpGetProbe("/healthz", "diag", 5, 6)).
		WithReadinessProbe(httpGetProbe("/readyz", "diag", 5, 12)).
		WithVolumeMounts(volumeMounts...)

	podSpec := corev1ac.PodSpec().
		WithAutomountServiceAccountToken(true).
		WithSecurityContext(corev1ac.PodSecurityContext().WithFSGroup(9807)).
		WithTerminationGracePeriodSeconds(30).
		WithServiceAccountName(agentName).
		WithContainers(container).
		WithVolumes(volumes...)

	if p.antiAffinityOn {
		podSpec = podSpec.WithAffinity(corev1ac.Affinity().
			WithPodAntiAffinity(corev1ac.PodAntiAffinity().
				WithPreferredDuringSchedulingIgnoredDuringExecution(
					corev1ac.WeightedPodAffinityTerm().
						WithWeight(50).
						WithPodAffinityTerm(corev1ac.PodAffinityTerm().
							WithLabelSelector(metav1ac.LabelSelector().
								WithMatchExpressions(metav1ac.LabelSelectorRequirement().
									WithKey("app").
									WithOperator(metav1.LabelSelectorOpIn).
									WithValues(agentName))).
							WithTopologyKey("kubernetes.io/hostname")))))
	}
	if p.topologySpread {
		podSpec = podSpec.WithTopologySpreadConstraints(
			corev1ac.TopologySpreadConstraint().
				WithMaxSkew(1).
				WithTopologyKey("kubernetes.io/hostname").
				WithWhenUnsatisfiable(corev1.ScheduleAnyway).
				WithLabelSelector(metav1ac.LabelSelector().WithMatchLabels(appSelector)),
			corev1ac.TopologySpreadConstraint().
				WithMaxSkew(1).
				WithTopologyKey("topology.kubernetes.io/zone").
				WithWhenUnsatisfiable(corev1.ScheduleAnyway).
				WithLabelSelector(metav1ac.LabelSelector().WithMatchLabels(appSelector)),
		)
	}

	ss := appsv1ac.StatefulSet(agentName, agentNamespace).
		WithLabels(appSelector).
		WithSpec(appsv1ac.StatefulSetSpec().
			WithServiceName(agentName).
			WithReplicas(p.replicaCount).
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(appSelector)).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithAnnotations(map[string]string{"checksum/config": p.configChecksum}).
				WithLabels(appSelector).
				WithSpec(podSpec)))

	_, err := clientset.AppsV1().StatefulSets(agentNamespace).Apply(ctx, ss, opts)
	return trace.Wrap(err)
}

func restrictedSecurityContext() *corev1ac.SecurityContextApplyConfiguration {
	return corev1ac.SecurityContext().
		WithAllowPrivilegeEscalation(false).
		WithCapabilities(corev1ac.Capabilities().WithDrop(corev1.Capability("ALL"))).
		WithReadOnlyRootFilesystem(true).
		WithRunAsNonRoot(true).
		WithRunAsUser(9807).
		WithSeccompProfile(corev1ac.SeccompProfile().WithType(corev1.SeccompProfileTypeRuntimeDefault))
}

func httpGetProbe(path, port string, initialDelay, failureThreshold int32) *corev1ac.ProbeApplyConfiguration {
	return corev1ac.Probe().
		WithHTTPGet(corev1ac.HTTPGetAction().
			WithPath(path).
			WithPort(intstr.FromString(port))).
		WithInitialDelaySeconds(initialDelay).
		WithPeriodSeconds(5).
		WithFailureThreshold(failureThreshold).
		WithTimeoutSeconds(1)
}

type updaterDeploymentParams struct {
	proxyAddr      string
	agentVersion   string
	releaseChannel string
	baseImage      string
}

func applyUpdaterServiceAccount(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	sa := corev1ac.ServiceAccount(agentName+"-updater", agentNamespace)
	_, err := clientset.CoreV1().ServiceAccounts(agentNamespace).Apply(ctx, sa, opts)
	return trace.Wrap(err)
}

func applyUpdaterRole(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	r := rbacv1ac.Role(agentName+"-updater", agentNamespace).
		WithRules(
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("pods").
				WithVerbs("get", "watch", "list", "delete"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("pods/status").
				WithVerbs("get", "watch", "list"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("secrets").
				WithVerbs("watch", "list"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("secrets").
				WithVerbs("get").
				WithResourceNames(agentName+"-shared-state"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("events").
				WithVerbs("create", "patch"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("configmaps").
				WithVerbs("create", "watch", "list"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("").
				WithResources("configmaps").
				WithVerbs("get", "update").
				WithResourceNames(agentName+"-updater"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("apps").
				WithResources("deployments", "statefulsets", "deployments/status", "statefulsets/status").
				WithVerbs("get", "watch", "list"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("apps").
				WithResources("deployments", "statefulsets").
				WithVerbs("update").
				WithResourceNames(agentName),
			rbacv1ac.PolicyRule().
				WithAPIGroups("coordination.k8s.io").
				WithResources("leases").
				WithVerbs("create"),
			rbacv1ac.PolicyRule().
				WithAPIGroups("coordination.k8s.io").
				WithResources("leases").
				WithVerbs("get", "update").
				WithResourceNames(agentName),
		)
	_, err := clientset.RbacV1().Roles(agentNamespace).Apply(ctx, r, opts)
	return trace.Wrap(err)
}

func applyUpdaterRoleBinding(ctx context.Context, clientset *kubernetes.Clientset, opts metav1.ApplyOptions) error {
	rb := rbacv1ac.RoleBinding(agentName+"-updater", agentNamespace).
		WithRoleRef(rbacv1ac.RoleRef().
			WithAPIGroup("rbac.authorization.k8s.io").
			WithKind("Role").
			WithName(agentName+"-updater")).
		WithSubjects(rbacv1ac.Subject().
			WithKind("ServiceAccount").
			WithName(agentName+"-updater").
			WithNamespace(agentNamespace))
	_, err := clientset.RbacV1().RoleBindings(agentNamespace).Apply(ctx, rb, opts)
	return trace.Wrap(err)
}

func applyUpdaterDeployment(ctx context.Context, clientset *kubernetes.Clientset, p updaterDeploymentParams, opts metav1.ApplyOptions) error {
	selector := map[string]string{"app": agentName + "-updater"}

	container := corev1ac.Container().
		WithName("kube-agent-updater").
		WithImage(kubeAgentUpdaterImage + ":" + p.agentVersion).
		WithImagePullPolicy(corev1.PullIfNotPresent).
		WithArgs(
			"--agent-name="+agentName,
			"--agent-namespace="+agentNamespace,
			"--base-image="+p.baseImage,
			"--version-server=https://"+p.proxyAddr+"/v1/webapi/automaticupgrades/channel",
			"--version-channel="+p.releaseChannel,
			"--proxy-address="+p.proxyAddr,
			"--update-group=default",
		).
		WithPorts(
			corev1ac.ContainerPort().WithName("metrics").WithContainerPort(8080).WithProtocol(corev1.ProtocolTCP),
			corev1ac.ContainerPort().WithName("healthz").WithContainerPort(8081).WithProtocol(corev1.ProtocolTCP),
		).
		WithLivenessProbe(httpGetProbe("/healthz", "healthz", 5, 6).WithTimeoutSeconds(5)).
		WithReadinessProbe(httpGetProbe("/readyz", "healthz", 5, 6).WithTimeoutSeconds(5)).
		WithSecurityContext(restrictedSecurityContext())

	podSpec := corev1ac.PodSpec().
		WithContainers(container).
		WithServiceAccountName(agentName + "-updater")

	deploy := appsv1ac.Deployment(agentName+"-updater", agentNamespace).
		WithLabels(selector).
		WithSpec(appsv1ac.DeploymentSpec().
			WithReplicas(1).
			WithSelector(metav1ac.LabelSelector().WithMatchLabels(selector)).
			WithTemplate(corev1ac.PodTemplateSpec().
				WithLabels(selector).
				WithSpec(podSpec)))

	_, err := clientset.AppsV1().Deployments(agentNamespace).Apply(ctx, deploy, opts)
	return trace.Wrap(err)
}
