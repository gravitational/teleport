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

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/tlsca"
)

func onCertInspect(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return trace.Wrap(err, "reading certificate file")
	}

	cert, err := tlsca.ParseCertificatePEM(data)
	if err != nil {
		return trace.Wrap(err, "parsing certificate")
	}

	identity, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return trace.Wrap(err, "extracting identity from certificate")
	}

	// X.509 fields
	fmt.Println("# X.509 Certificate")
	fmt.Printf("  Subject:    %s\n", cert.Subject.CommonName)
	fmt.Printf("  Issuer:     %s\n", cert.Issuer.CommonName)
	fmt.Printf("  Serial:     %s\n", cert.SerialNumber)
	fmt.Printf("  Not Before: %s\n", cert.NotBefore.Format(time.RFC3339))
	fmt.Printf("  Not After:  %s (%s)\n", cert.NotAfter.Format(time.RFC3339), time.Until(cert.NotAfter).Truncate(time.Second))
	if len(cert.DNSNames) > 0 {
		fmt.Printf("  DNS SANs:   %s\n", strings.Join(cert.DNSNames, ", "))
	}
	for _, ip := range cert.IPAddresses {
		fmt.Printf("  IP SAN:     %s\n", ip)
	}
	for _, uri := range cert.URIs {
		fmt.Printf("  URI SAN:    %s\n", uri)
	}

	// Teleport identity fields
	fmt.Println("\n# Teleport Identity")
	fmt.Printf("  Username:         %s\n", identity.Username)
	if identity.BotName != "" {
		fmt.Printf("  Bot Name:         %s\n", identity.BotName)
	}
	if identity.BotInstanceID != "" {
		fmt.Printf("  Bot Instance ID:  %s\n", identity.BotInstanceID)
	}
	if identity.TeleportCluster != "" {
		fmt.Printf("  Cluster:          %s\n", identity.TeleportCluster)
	}
	if len(identity.Groups) > 0 {
		fmt.Printf("  Roles:            %s\n", strings.Join(identity.Groups, ", "))
	}
	if len(identity.SystemRoles) > 0 {
		fmt.Printf("  System Roles:     %s\n", strings.Join(identity.SystemRoles, ", "))
	}
	if identity.Renewable {
		fmt.Printf("  Renewable:        true\n")
	}
	if identity.Generation > 0 {
		fmt.Printf("  Generation:       %d\n", identity.Generation)
	}
	if identity.DisallowReissue {
		fmt.Printf("  Disallow Reissue: true\n")
	}
	if identity.JoinToken != "" {
		fmt.Printf("  Join Token:       %s\n", identity.JoinToken)
	}
	if identity.Impersonator != "" {
		fmt.Printf("  Impersonator:     %s\n", identity.Impersonator)
	}
	if identity.RouteToCluster != "" {
		fmt.Printf("  Route To Cluster: %s\n", identity.RouteToCluster)
	}
	if len(identity.Principals) > 0 {
		fmt.Printf("  Principals:       %s\n", strings.Join(identity.Principals, ", "))
	}
	if len(identity.Usage) > 0 {
		fmt.Printf("  Usage:            %s\n", strings.Join(identity.Usage, ", "))
	}
	if len(identity.ActiveRequests) > 0 {
		fmt.Printf("  Active Requests:  %s\n", strings.Join(identity.ActiveRequests, ", "))
	}
	if identity.MFAVerified != "" {
		fmt.Printf("  MFA Verified:     %s\n", identity.MFAVerified)
	}
	if identity.LoginIP != "" {
		fmt.Printf("  Login IP:         %s\n", identity.LoginIP)
	}
	if identity.PinnedIP != "" {
		fmt.Printf("  Pinned IP:        %s\n", identity.PinnedIP)
	}
	if !identity.PreviousIdentityExpires.IsZero() {
		fmt.Printf("  Prev ID Expires:  %s\n", identity.PreviousIdentityExpires.Format(time.RFC3339))
	}

	// Routing
	if identity.KubernetesCluster != "" {
		fmt.Printf("  K8s Cluster:      %s\n", identity.KubernetesCluster)
	}
	if len(identity.KubernetesGroups) > 0 {
		fmt.Printf("  K8s Groups:       %s\n", strings.Join(identity.KubernetesGroups, ", "))
	}
	if len(identity.KubernetesUsers) > 0 {
		fmt.Printf("  K8s Users:        %s\n", strings.Join(identity.KubernetesUsers, ", "))
	}
	if len(identity.DatabaseNames) > 0 {
		fmt.Printf("  DB Names:         %s\n", strings.Join(identity.DatabaseNames, ", "))
	}
	if len(identity.DatabaseUsers) > 0 {
		fmt.Printf("  DB Users:         %s\n", strings.Join(identity.DatabaseUsers, ", "))
	}
	if identity.RouteToDatabase.ServiceName != "" {
		fmt.Printf("  Route To DB:      %s (protocol: %s, user: %s)\n",
			identity.RouteToDatabase.ServiceName, identity.RouteToDatabase.Protocol, identity.RouteToDatabase.Username)
	}
	if identity.RouteToApp.SessionID != "" {
		fmt.Printf("  Route To App:     %s (session: %s)\n",
			identity.RouteToApp.Name, identity.RouteToApp.SessionID)
	}

	// Cloud provider identities
	if len(identity.AWSRoleARNs) > 0 {
		fmt.Printf("  AWS Role ARNs:    %s\n", strings.Join(identity.AWSRoleARNs, ", "))
	}
	if len(identity.AzureIdentities) > 0 {
		fmt.Printf("  Azure Identities: %s\n", strings.Join(identity.AzureIdentities, ", "))
	}
	if len(identity.GCPServiceAccounts) > 0 {
		fmt.Printf("  GCP SAs:          %s\n", strings.Join(identity.GCPServiceAccounts, ", "))
	}

	// Scopes
	if identity.AgentScope != "" {
		fmt.Printf("\n# Scope\n")
		fmt.Printf("  Agent Scope:      %s\n", identity.AgentScope)
	}
	if identity.ScopePin != nil {
		if identity.AgentScope == "" {
			fmt.Printf("\n# Scope\n")
		}
		fmt.Printf("  Scope Pin:\n")
		fmt.Printf("    Scope: %s\n", identity.ScopePin.Scope)
		if len(identity.ScopePin.Assignments) > 0 {
			fmt.Printf("    Assignments:\n")
			for scope, assignments := range identity.ScopePin.Assignments {
				fmt.Printf("      %s: %s\n", scope, strings.Join(assignments.Roles, ", "))
			}
		}
	}

	return nil
}
