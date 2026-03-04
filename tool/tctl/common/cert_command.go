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

package common

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/scopes/pinning"
	"github.com/gravitational/teleport/lib/tlsca"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

type certCommand struct {
	inspect  *kingpin.CmdClause
	certPath string
	format   string
}

func (c *certCommand) Initialize(authCmd *kingpin.CmdClause) {
	c.inspect = authCmd.Command("inspect", "Inspect a Teleport TLS certificate and display the embedded identity.")
	c.inspect.Arg("cert", "Path to a PEM-encoded X509 certificate file.").Required().StringVar(&c.certPath)
	c.inspect.Flag("format", "Output format: text, json, or yaml.").Default(teleport.Text).EnumVar(&c.format, teleport.Text, teleport.JSON, teleport.YAML)
}

func (c *certCommand) TryRun(ctx context.Context, cmd string, _ commonclient.InitFunc) (match bool, err error) {
	if c.inspect.FullCommand() != cmd {
		return false, nil
	}
	return true, trace.Wrap(c.run(ctx, os.Stdout))
}

func (c *certCommand) run(_ context.Context, w io.Writer) error {
	pemBytes, err := os.ReadFile(c.certPath)
	if err != nil {
		return trace.Wrap(err, "reading certificate file")
	}

	cert, err := tlsca.ParseCertificatePEM(pemBytes)
	if err != nil {
		return trace.Wrap(err, "parsing certificate")
	}

	id, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return trace.Wrap(err, "extracting Teleport identity from certificate")
	}

	switch c.format {
	case teleport.Text:
		printCertText(w, cert, id)
	case teleport.JSON:
		return trace.Wrap(printCertJSON(w, cert, id))
	case teleport.YAML:
		return trace.Wrap(printCertYAML(w, cert, id))
	}
	return nil
}

func printCertText(w io.Writer, cert *x509.Certificate, id *tlsca.Identity) {
	fmt.Fprintf(w, "X509 Certificate:\n")
	fmt.Fprintf(w, "  Serial:               %s\n", cert.SerialNumber)
	fmt.Fprintf(w, "  Not Before:           %s\n", cert.NotBefore.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "  Not After:            %s\n", cert.NotAfter.UTC().Format("2006-01-02 15:04:05 UTC"))
	fmt.Fprintf(w, "  Issuer:               %s\n", cert.Issuer.CommonName)
	fmt.Fprintf(w, "  Subject:              %s\n", cert.Subject.CommonName)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "Teleport Identity:\n")
	printField(w, "Username", id.Username)
	printField(w, "Cluster", id.TeleportCluster)
	printField(w, "Origin Cluster", id.OriginClusterName)
	if !id.Expires.IsZero() {
		printField(w, "Expires", id.Expires.UTC().Format("2006-01-02 15:04:05 UTC"))
	}

	// Roles vs scope pin (mutually exclusive).
	if id.ScopePin != nil {
		printField(w, "Scope", id.ScopePin.GetScope())
		if tree := id.ScopePin.GetAssignmentTree(); tree != nil {
			fmt.Fprintf(w, "  Scoped Roles:\n")
			fmt.Fprint(w, pinning.FormatAssignmentTree(tree, "    "))
		}
	} else if len(id.Groups) > 0 {
		printField(w, "Roles", strings.Join(id.Groups, ", "))
	}

	printField(w, "Agent Scope", id.AgentScope)
	printSlice(w, "Logins", id.Principals)
	printSlice(w, "Kubernetes Users", id.KubernetesUsers)
	printSlice(w, "Kubernetes Groups", id.KubernetesGroups)
	printField(w, "Kubernetes Cluster", id.KubernetesCluster)
	printField(w, "Route To Cluster", id.RouteToCluster)
	printField(w, "Pinned IP", id.PinnedIP)
	printField(w, "Login IP", id.LoginIP)
	printField(w, "MFA Verified", id.MFAVerified)
	printField(w, "Impersonator", id.Impersonator)
	printSlice(w, "Active Requests", id.ActiveRequests)
	if id.DisallowReissue {
		printField(w, "Disallow Reissue", "true")
	}
	if id.Renewable {
		printField(w, "Renewable", "true")
	}
	printField(w, "Bot Name", id.BotName)
	printField(w, "Bot Instance ID", id.BotInstanceID)
	printField(w, "Join Token", id.JoinToken)
	if id.Generation > 0 {
		printField(w, "Generation", fmt.Sprintf("%d", id.Generation))
	}
	if id.PrivateKeyPolicy != "" {
		printField(w, "Private Key Policy", string(id.PrivateKeyPolicy))
	}
	printSlice(w, "System Roles", id.SystemRoles)
	printSlice(w, "Usage", id.Usage)
	if id.UserType != "" {
		printField(w, "User Type", string(id.UserType))
	}
	if !id.PreviousIdentityExpires.IsZero() {
		printField(w, "Prev Identity Expires", id.PreviousIdentityExpires.UTC().Format("2006-01-02 15:04:05 UTC"))
	}
	printField(w, "Connection Diag ID", id.ConnectionDiagnosticID)
	printField(w, "Immutable Label Hash", id.ImmutableLabelHash)

	// Route To App (only if populated).
	if id.RouteToApp != (tlsca.RouteToApp{}) {
		fmt.Fprintf(w, "\n  Route To App:\n")
		printSubField(w, "Name", id.RouteToApp.Name)
		printSubField(w, "Public Addr", id.RouteToApp.PublicAddr)
		printSubField(w, "Cluster", id.RouteToApp.ClusterName)
		printSubField(w, "Session ID", id.RouteToApp.SessionID)
		printSubField(w, "URI", id.RouteToApp.URI)
		if id.RouteToApp.TargetPort != 0 {
			printSubField(w, "Target Port", fmt.Sprintf("%d", id.RouteToApp.TargetPort))
		}
		printSubField(w, "AWS Role ARN", id.RouteToApp.AWSRoleARN)
		printSubField(w, "Azure Identity", id.RouteToApp.AzureIdentity)
		printSubField(w, "GCP Service Account", id.RouteToApp.GCPServiceAccount)
	}

	// Route To Database (only if populated).
	if !id.RouteToDatabase.Empty() {
		fmt.Fprintf(w, "\n  Route To Database:\n")
		printSubField(w, "Service", id.RouteToDatabase.ServiceName)
		printSubField(w, "Protocol", id.RouteToDatabase.Protocol)
		printSubField(w, "Username", id.RouteToDatabase.Username)
		printSubField(w, "Database", id.RouteToDatabase.Database)
		if len(id.RouteToDatabase.Roles) > 0 {
			printSubField(w, "Roles", strings.Join(id.RouteToDatabase.Roles, ", "))
		}
	}

	// Device Extensions (only if populated).
	if id.DeviceExtensions != (tlsca.DeviceExtensions{}) {
		fmt.Fprintf(w, "\n  Device Extensions:\n")
		printSubField(w, "Device ID", id.DeviceExtensions.DeviceID)
		printSubField(w, "Asset Tag", id.DeviceExtensions.AssetTag)
		printSubField(w, "Credential ID", id.DeviceExtensions.CredentialID)
	}

	// Cloud provider identities.
	printSlice(w, "AWS Role ARNs", id.AWSRoleARNs)
	printSlice(w, "Azure Identities", id.AzureIdentities)
	printSlice(w, "GCP Service Accounts", id.GCPServiceAccounts)

	// Database names/users.
	printSlice(w, "Database Names", id.DatabaseNames)
	printSlice(w, "Database Users", id.DatabaseUsers)
}

// printField prints a labeled field if the value is non-empty.
func printField(w io.Writer, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(w, "  %-22s%s\n", label+":", value)
}

// printSubField prints a labeled field indented under a section.
func printSubField(w io.Writer, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(w, "    %-20s%s\n", label+":", value)
}

// printSlice prints a labeled comma-separated list if the slice is non-empty.
func printSlice(w io.Writer, label string, values []string) {
	if len(values) == 0 {
		return
	}
	printField(w, label, strings.Join(values, ", "))
}

// certInspectOutput is the structured output for JSON/YAML format.
type certInspectOutput struct {
	Certificate certInfo          `json:"certificate" yaml:"certificate"`
	Identity    identityInfo      `json:"identity" yaml:"identity"`
}

type certInfo struct {
	Serial    string `json:"serial" yaml:"serial"`
	NotBefore string `json:"not_before" yaml:"not_before"`
	NotAfter  string `json:"not_after" yaml:"not_after"`
	Issuer    string `json:"issuer" yaml:"issuer"`
	Subject   string `json:"subject" yaml:"subject"`
}

type identityInfo struct {
	Username              string            `json:"username,omitempty" yaml:"username,omitempty"`
	TeleportCluster       string            `json:"teleport_cluster,omitempty" yaml:"teleport_cluster,omitempty"`
	OriginClusterName     string            `json:"origin_cluster_name,omitempty" yaml:"origin_cluster_name,omitempty"`
	Expires               string            `json:"expires,omitempty" yaml:"expires,omitempty"`
	Roles                 []string          `json:"roles,omitempty" yaml:"roles,omitempty"`
	Scope                 string            `json:"scope,omitempty" yaml:"scope,omitempty"`
	ScopedRoles           map[string]map[string][]string `json:"scoped_roles,omitempty" yaml:"scoped_roles,omitempty"`
	AgentScope            string            `json:"agent_scope,omitempty" yaml:"agent_scope,omitempty"`
	Logins                []string          `json:"logins,omitempty" yaml:"logins,omitempty"`
	KubernetesUsers       []string          `json:"kubernetes_users,omitempty" yaml:"kubernetes_users,omitempty"`
	KubernetesGroups      []string          `json:"kubernetes_groups,omitempty" yaml:"kubernetes_groups,omitempty"`
	KubernetesCluster     string            `json:"kubernetes_cluster,omitempty" yaml:"kubernetes_cluster,omitempty"`
	RouteToCluster        string            `json:"route_to_cluster,omitempty" yaml:"route_to_cluster,omitempty"`
	PinnedIP              string            `json:"pinned_ip,omitempty" yaml:"pinned_ip,omitempty"`
	LoginIP               string            `json:"login_ip,omitempty" yaml:"login_ip,omitempty"`
	MFAVerified           string            `json:"mfa_verified,omitempty" yaml:"mfa_verified,omitempty"`
	Impersonator          string            `json:"impersonator,omitempty" yaml:"impersonator,omitempty"`
	ActiveRequests        []string          `json:"active_requests,omitempty" yaml:"active_requests,omitempty"`
	DisallowReissue       bool              `json:"disallow_reissue,omitempty" yaml:"disallow_reissue,omitempty"`
	Renewable             bool              `json:"renewable,omitempty" yaml:"renewable,omitempty"`
	BotName               string            `json:"bot_name,omitempty" yaml:"bot_name,omitempty"`
	BotInstanceID         string            `json:"bot_instance_id,omitempty" yaml:"bot_instance_id,omitempty"`
	JoinToken             string            `json:"join_token,omitempty" yaml:"join_token,omitempty"`
	Generation            uint64            `json:"generation,omitempty" yaml:"generation,omitempty"`
	PrivateKeyPolicy      string            `json:"private_key_policy,omitempty" yaml:"private_key_policy,omitempty"`
	SystemRoles           []string          `json:"system_roles,omitempty" yaml:"system_roles,omitempty"`
	Usage                 []string          `json:"usage,omitempty" yaml:"usage,omitempty"`
	UserType              string            `json:"user_type,omitempty" yaml:"user_type,omitempty"`
	RouteToApp            *routeToAppInfo   `json:"route_to_app,omitempty" yaml:"route_to_app,omitempty"`
	RouteToDatabase       *routeToDBInfo    `json:"route_to_database,omitempty" yaml:"route_to_database,omitempty"`
	DeviceExtensions      *deviceExtInfo    `json:"device_extensions,omitempty" yaml:"device_extensions,omitempty"`
	AWSRoleARNs           []string          `json:"aws_role_arns,omitempty" yaml:"aws_role_arns,omitempty"`
	AzureIdentities       []string          `json:"azure_identities,omitempty" yaml:"azure_identities,omitempty"`
	GCPServiceAccounts    []string          `json:"gcp_service_accounts,omitempty" yaml:"gcp_service_accounts,omitempty"`
	DatabaseNames         []string          `json:"database_names,omitempty" yaml:"database_names,omitempty"`
	DatabaseUsers         []string          `json:"database_users,omitempty" yaml:"database_users,omitempty"`
	ConnectionDiagID      string            `json:"connection_diagnostic_id,omitempty" yaml:"connection_diagnostic_id,omitempty"`
	ImmutableLabelHash    string            `json:"immutable_label_hash,omitempty" yaml:"immutable_label_hash,omitempty"`
}

type routeToAppInfo struct {
	Name              string `json:"name,omitempty" yaml:"name,omitempty"`
	PublicAddr        string `json:"public_addr,omitempty" yaml:"public_addr,omitempty"`
	ClusterName       string `json:"cluster_name,omitempty" yaml:"cluster_name,omitempty"`
	SessionID         string `json:"session_id,omitempty" yaml:"session_id,omitempty"`
	URI               string `json:"uri,omitempty" yaml:"uri,omitempty"`
	TargetPort        int    `json:"target_port,omitempty" yaml:"target_port,omitempty"`
	AWSRoleARN        string `json:"aws_role_arn,omitempty" yaml:"aws_role_arn,omitempty"`
	AzureIdentity     string `json:"azure_identity,omitempty" yaml:"azure_identity,omitempty"`
	GCPServiceAccount string `json:"gcp_service_account,omitempty" yaml:"gcp_service_account,omitempty"`
}

type routeToDBInfo struct {
	ServiceName string   `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	Protocol    string   `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Username    string   `json:"username,omitempty" yaml:"username,omitempty"`
	Database    string   `json:"database,omitempty" yaml:"database,omitempty"`
	Roles       []string `json:"roles,omitempty" yaml:"roles,omitempty"`
}

type deviceExtInfo struct {
	DeviceID     string `json:"device_id,omitempty" yaml:"device_id,omitempty"`
	AssetTag     string `json:"asset_tag,omitempty" yaml:"asset_tag,omitempty"`
	CredentialID string `json:"credential_id,omitempty" yaml:"credential_id,omitempty"`
}

func buildStructuredOutput(cert *x509.Certificate, id *tlsca.Identity) certInspectOutput {
	out := certInspectOutput{
		Certificate: certInfo{
			Serial:    cert.SerialNumber.String(),
			NotBefore: cert.NotBefore.UTC().Format("2006-01-02T15:04:05Z"),
			NotAfter:  cert.NotAfter.UTC().Format("2006-01-02T15:04:05Z"),
			Issuer:    cert.Issuer.CommonName,
			Subject:   cert.Subject.CommonName,
		},
		Identity: identityInfo{
			Username:           id.Username,
			TeleportCluster:    id.TeleportCluster,
			OriginClusterName:  id.OriginClusterName,
			Roles:              id.Groups,
			AgentScope:         id.AgentScope,
			Logins:             id.Principals,
			KubernetesUsers:    id.KubernetesUsers,
			KubernetesGroups:   id.KubernetesGroups,
			KubernetesCluster:  id.KubernetesCluster,
			RouteToCluster:     id.RouteToCluster,
			PinnedIP:           id.PinnedIP,
			LoginIP:            id.LoginIP,
			MFAVerified:        id.MFAVerified,
			Impersonator:       id.Impersonator,
			ActiveRequests:     id.ActiveRequests,
			DisallowReissue:    id.DisallowReissue,
			Renewable:          id.Renewable,
			BotName:            id.BotName,
			BotInstanceID:      id.BotInstanceID,
			JoinToken:          id.JoinToken,
			Generation:         id.Generation,
			PrivateKeyPolicy:   string(id.PrivateKeyPolicy),
			SystemRoles:        id.SystemRoles,
			Usage:              id.Usage,
			AWSRoleARNs:        id.AWSRoleARNs,
			AzureIdentities:    id.AzureIdentities,
			GCPServiceAccounts: id.GCPServiceAccounts,
			DatabaseNames:      id.DatabaseNames,
			DatabaseUsers:      id.DatabaseUsers,
			ConnectionDiagID:   id.ConnectionDiagnosticID,
			ImmutableLabelHash: id.ImmutableLabelHash,
		},
	}

	if !id.Expires.IsZero() {
		out.Identity.Expires = id.Expires.UTC().Format("2006-01-02T15:04:05Z")
	}

	if id.UserType != "" {
		out.Identity.UserType = string(id.UserType)
	}

	if id.ScopePin != nil {
		out.Identity.Scope = id.ScopePin.GetScope()
		if tree := id.ScopePin.GetAssignmentTree(); tree != nil {
			out.Identity.ScopedRoles = pinning.AssignmentTreeIntoMap(tree)
		}
	}

	if id.RouteToApp != (tlsca.RouteToApp{}) {
		out.Identity.RouteToApp = &routeToAppInfo{
			Name:              id.RouteToApp.Name,
			PublicAddr:        id.RouteToApp.PublicAddr,
			ClusterName:       id.RouteToApp.ClusterName,
			SessionID:         id.RouteToApp.SessionID,
			URI:               id.RouteToApp.URI,
			TargetPort:        id.RouteToApp.TargetPort,
			AWSRoleARN:        id.RouteToApp.AWSRoleARN,
			AzureIdentity:     id.RouteToApp.AzureIdentity,
			GCPServiceAccount: id.RouteToApp.GCPServiceAccount,
		}
	}

	if !id.RouteToDatabase.Empty() {
		out.Identity.RouteToDatabase = &routeToDBInfo{
			ServiceName: id.RouteToDatabase.ServiceName,
			Protocol:    id.RouteToDatabase.Protocol,
			Username:    id.RouteToDatabase.Username,
			Database:    id.RouteToDatabase.Database,
			Roles:       id.RouteToDatabase.Roles,
		}
	}

	if id.DeviceExtensions != (tlsca.DeviceExtensions{}) {
		out.Identity.DeviceExtensions = &deviceExtInfo{
			DeviceID:     id.DeviceExtensions.DeviceID,
			AssetTag:     id.DeviceExtensions.AssetTag,
			CredentialID: id.DeviceExtensions.CredentialID,
		}
	}

	return out
}

func printCertJSON(w io.Writer, cert *x509.Certificate, id *tlsca.Identity) error {
	out := buildStructuredOutput(cert, id)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return trace.Wrap(enc.Encode(out))
}

func printCertYAML(w io.Writer, cert *x509.Certificate, id *tlsca.Identity) error {
	out := buildStructuredOutput(cert, id)
	data, err := yaml.Marshal(out)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(data)
	return trace.Wrap(err)
}
