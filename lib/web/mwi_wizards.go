package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/tfgen/transform"
)

// machineIDWizard generates IaC code for the Machine Identity CI/CD wizards.
func (h *Handler) machineIDWizard(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext) (any, error) {
	var req machineIDWizardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, trace.Wrap(err)
	}

	// Note: we currently only support deploying from GitHub Actions to Kubernetes
	// but this endpoint will support other sources and destinations in the future.
	switch {
	case req.SourceType != "github":
		return nil, trace.BadParameter("source_type must be one of: [github]")
	case req.DestinationType != "kubernetes":
		return nil, trace.BadParameter("destination_type must be one of: [kubernetes]")
	}

	if req.GitHub == nil {
		// Default to *something* so the generated code isn't completely broken.
		req.GitHub = &machineIDWizardRequestGitHub{
			Repository: "repository",
			Owner:      "organization",
		}
	}

	namePrefix := fmt.Sprintf("gha-%s-%s", req.GitHub.Owner, req.GitHub.Repository)

	// Role resource.
	role := &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: fmt.Sprintf("%s-kube-access", namePrefix),
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeGroups: []string{"{{internal.kubernetes_groups}}"},
				KubeUsers:  []string{"{{internal.kubernetes_users}}"},
			},
		},
	}
	var roleOpts []tfgen.GenerateOpt
	if req.Kubernetes != nil {
		role.Spec.Allow.KubernetesLabels = req.Kubernetes.Labels
		role.Spec.Allow.KubernetesResources = req.Kubernetes.Resources
	} else {
		roleOpts = append(roleOpts, tfgen.WithFieldComment("spec.allow.kubernetes_labels", "kubernetes_labels will be added in the next steps."))
	}

	// Bot resource.
	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: namePrefix,
		},
		Spec: &machineidv1.BotSpec{},
	}
	botOpts := []tfgen.GenerateOpt{
		tfgen.WithFieldTransform("spec.traits", transform.BotTraits),
	}

	if req.Kubernetes == nil {
		botOpts = append(botOpts,
			tfgen.WithFieldComment("spec.traits", "kubernetes_groups and kubernetes_users will be added in the next step."),
		)
	} else {
		if len(req.Kubernetes.Groups) != 0 {
			bot.Spec.Traits = append(bot.Spec.Traits, &machineidv1.Trait{
				Name:   "kubernetes_groups",
				Values: req.Kubernetes.Groups,
			})
		}
		if len(req.Kubernetes.Users) != 0 {
			bot.Spec.Traits = append(bot.Spec.Traits, &machineidv1.Trait{
				Name:   "kubernetes_users",
				Values: req.Kubernetes.Users,
			})
		}
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
				Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
					{
						Repository:      fmt.Sprintf("%s/%s", req.GitHub.Owner, req.GitHub.Repository),
						RepositoryOwner: req.GitHub.Owner,
						Workflow:        req.GitHub.Workflow,
						Environment:     req.GitHub.Environment,
						Actor:           req.GitHub.Actor,
						Ref:             req.GitHub.Ref,
						RefType:         req.GitHub.RefType,
					},
				},
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

	return machineIDGHAK8sWizardResponse{
		Terraform: fmt.Sprintf(machineIDGHAK8sWizardTerraformTemplate, roleTF, botTF, tokenTF),
	}, nil
}

//go:embed templates/machine-id-gha-k8s-wizard-terraform.tmpl
var machineIDGHAK8sWizardTerraformTemplate string

type machineIDWizardRequest struct {
	SourceType      string                            `json:"source_type"`
	DestinationType string                            `json:"destination_type"`
	GitHub          *machineIDWizardRequestGitHub     `json:"github"`
	Kubernetes      *machineIDWizardRequestKubernetes `json:"kubernetes`
}

type machineIDWizardRequestGitHub struct {
	Repository           string `json:"repository"`
	Owner                string `json:"owner"`
	Workflow             string `json:"workflow"`
	Environment          string `json:"environment"`
	Actor                string `json:"actor"`
	Ref                  string `json:"ref"`
	RefType              string `json:"ref_type"`
	EnterpriseServerHost string `json:"enterprise_server_host"`
	EnterpriseSlug       string `json:"enterprise_slug"`
	StaticJWKS           string `json:"static_jwks"`
}

type machineIDWizardRequestKubernetes struct {
	Labels    types.Labels               `json:"labels"`
	Groups    []string                   `json:"groups"`
	Users     []string                   `json:"users"`
	Resources []types.KubernetesResource `json:"resources"`
}

type machineIDGHAK8sWizardResponse struct {
	Terraform string `json:"terraform"`
}
