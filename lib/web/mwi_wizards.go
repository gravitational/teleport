package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/tfgen/transform"
	"github.com/gravitational/teleport/lib/utils/slices"
)

// machineIDWizardGenerateIaC generates IaC code for the Machine Identity CI/CD wizards.
func (h *Handler) machineIDWizardGenerateIaC(_ http.ResponseWriter, r *http.Request, _ httprouter.Params, _ *SessionContext, _ reversetunnelclient.Cluster) (any, error) {
	var req machineIDWizardGenerateIaCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.Wrap(err)
	}

	switch {
	// We currently only support deploying from GitHub Actions to Kubernetes but
	// this endpoint will support other sources and destinations in the future.
	case req.SourceType != "github":
		return nil, trace.BadParameter("source_type must be one of: [github]")
	case req.DestinationType != "kubernetes":
		return nil, trace.BadParameter("destination_type must be one of: [kubernetes]")
	// We also currently only support a single repository, but we may support
	// multiple in the future, so the allow field is a slice.
	case req.GitHub != nil && len(req.GitHub.Allow) != 1:
		return nil, trace.BadParameter("github.allow: must contain exactly one item")
	}

	if req.GitHub == nil {
		// Default to *something* so the generated code isn't completely broken.
		req.GitHub = &machineIDWizardRequestGitHub{
			Allow: []machineIDWizardRequestGitHubAllow{
				{
					Repository: "repository",
					Owner:      "organization",
				},
			},
		}
	}

	namePrefix := fmt.Sprintf(
		"gha-%s-%s",
		req.GitHub.Allow[0].Owner,
		req.GitHub.Allow[0].Repository,
	)

	// Role resource.
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: fmt.Sprintf("%s-kube-access", namePrefix),
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{},
		},
	}
	var roleOpts []tfgen.GenerateOpt
	if req.Kubernetes != nil {
		role.Spec.Allow.KubernetesLabels = req.Kubernetes.Labels
		role.Spec.Allow.KubernetesResources = req.Kubernetes.Resources
		role.Spec.Allow.KubeGroups = req.Kubernetes.Groups
		role.Spec.Allow.KubeUsers = req.Kubernetes.Users
	} else {
		roleOpts = append(roleOpts, tfgen.WithFieldComment("spec.allow.kubernetes_labels", "kubernetes_labels will be added in the next step."))
		roleOpts = append(roleOpts, tfgen.WithFieldComment("spec.allow.kubernetes_groups", "kubernetes_groups will be added in the next step."))
	}

	// Bot resource.
	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: namePrefix,
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{role.GetName()},
		},
	}
	botOpts := []tfgen.GenerateOpt{
		tfgen.WithFieldTransform("spec.traits", transform.BotTraits),
	}

	// Join token resource.
	token := &types.ProvisionTokenV2{
		Kind:    types.KindToken,
		Version: types.V2,
		Metadata: types.Metadata{
			Name: namePrefix,
		},
		Spec: types.ProvisionTokenSpecV2{
			Roles:      []types.SystemRole{types.RoleBot},
			JoinMethod: types.JoinMethodGitHub,
			BotName:    namePrefix,
			GitHub: &types.ProvisionTokenSpecV2GitHub{
				Allow: slices.Map(req.GitHub.Allow, func(allow machineIDWizardRequestGitHubAllow) *types.ProvisionTokenSpecV2GitHub_Rule {
					return &types.ProvisionTokenSpecV2GitHub_Rule{
						Repository:      fmt.Sprintf("%s/%s", allow.Owner, allow.Repository),
						RepositoryOwner: allow.Owner,
						Workflow:        allow.Workflow,
						Environment:     allow.Environment,
						Actor:           allow.Actor,
						Ref:             allow.Ref,
						RefType:         allow.RefType,
					}
				}),
				EnterpriseServerHost: req.GitHub.EnterpriseServerHost,
				EnterpriseSlug:       req.GitHub.EnterpriseSlug,
				StaticJWKS:           req.GitHub.StaticJWKS,
			},
		},
	}

	roleTF, err := tfgen.Generate(role, roleOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	botTF, err := tfgen.Generate(bot, botOpts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenTF, err := tfgen.Generate(token, tfgen.WithResourceType("teleport_provision_token"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return machineIDGHAK8sWizardGenerateIaCResponse{
		Terraform: fmt.Sprintf(
			machineIDGHAK8sWizardTerraformTemplate,
			roleTF,
			botTF,
			tokenTF,
			h.terraformProviderConfig(),
		),
	}, nil
}

//go:embed templates/terraform-provider.tf.tmpl
var terraformProviderTemplate string

// terraformProviderConfig returns base configuration for the Teleport terraform
// provider.
func (h *Handler) terraformProviderConfig() string {
	return fmt.Sprintf(
		terraformProviderTemplate,
		teleport.SemVer().Major,
		h.PublicProxyAddr(),
	)
}

//go:embed templates/machine-id-gha-k8s-wizard.tf.tmpl
var machineIDGHAK8sWizardTerraformTemplate string

type machineIDWizardGenerateIaCRequest struct {
	SourceType      string                            `json:"source_type"`
	DestinationType string                            `json:"destination_type"`
	GitHub          *machineIDWizardRequestGitHub     `json:"github"`
	Kubernetes      *machineIDWizardRequestKubernetes `json:"kubernetes`
}

type machineIDWizardRequestGitHub struct {
	Allow                []machineIDWizardRequestGitHubAllow `json:"allow"`
	EnterpriseServerHost string                              `json:"enterprise_server_host"`
	EnterpriseSlug       string                              `json:"enterprise_slug"`
	StaticJWKS           string                              `json:"static_jwks"`
}

type machineIDWizardRequestGitHubAllow struct {
	Repository  string `json:"repository"`
	Owner       string `json:"owner"`
	Workflow    string `json:"workflow"`
	Environment string `json:"environment"`
	Actor       string `json:"actor"`
	Ref         string `json:"ref"`
	RefType     string `json:"ref_type"`
}

type machineIDWizardRequestKubernetes struct {
	Labels    types.Labels               `json:"labels"`
	Groups    []string                   `json:"groups"`
	Users     []string                   `json:"users"`
	Resources []types.KubernetesResource `json:"resources"`
}

type machineIDGHAK8sWizardGenerateIaCResponse struct {
	Terraform string `json:"terraform"`
}
