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
	"io"
	"os"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	// protoc generators are programs that read a
	// google.protobuf.compiler.CodeGeneratorRequest message from stdin and
	// output a google.protobuf.compiler.CodeGeneratorResponse message to
	// stdout; we're interested in tweaking the output of protoc-gen-go but
	// there's no supported way to embed protoc-gen-go in code, so instead we
	// act as a filter to its output, reading the original
	// CodeGeneratorResponse, tweaking it, and writing the modified response to
	// stdout

	inBuf, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	cgr := new(pluginpb.CodeGeneratorResponse)
	if err := proto.Unmarshal(inBuf, cgr); err != nil {
		panic(err)
	}

	if cgr.GetError() != "" {
		// code generation failed, forward the response as is
		if _, err := os.Stdout.Write(inBuf); err != nil {
			panic(err)
		}
		return
	}

	for _, file := range cgr.GetFile() {
		if file.Name == nil {
			panic("got chunked output from protoc-gen-go")
		}
		if file.InsertionPoint != nil {
			panic("got insertion point in file from protoc-gen-go")
		}
		if !strings.HasSuffix(file.GetName(), ".pb.go") {
			panic("got file without .pb.go suffix from protoc-gen-go")
		}

		prefix, isProtoOpaque := strings.CutSuffix(file.GetName(), "_protoopaque.pb.go")
		if !isProtoOpaque {
			prefix = strings.TrimSuffix(file.GetName(), ".pb.go")
		}

		defaultHybrid := defaultHybridFromPrefix(prefix)

		if isProtoOpaque {
			before, after, ok := strings.Cut(file.GetContent(), "//go:build protoopaque\n")
			if !ok {
				panic("got protoopaque file without protoopaque built tag")
			}

			proto.Reset(file)

			newName := prefix + "_protoopaque.pb.go"
			file.Name = &newName

			var newContent string
			if defaultHybrid {
				newContent = before + "//go:build teleport_protoopaque\n" + after
			} else {
				newContent = before + "//go:build !teleport_protohybrid\n" + after
			}
			file.Content = &newContent
		} else {
			before, after, ok := strings.Cut(file.GetContent(), "//go:build !protoopaque\n")
			if !ok {
				panic("got non-protoopaque file without !protoopaque built tag")
			}

			proto.Reset(file)

			newName := prefix + "_protohybrid.pb.go"
			file.Name = &newName

			var newContent string
			if defaultHybrid {
				newContent = before + "//go:build !teleport_protoopaque\n" + after
			} else {
				newContent = before + "//go:build teleport_protohybrid\n" + after
			}
			file.Content = &newContent
		}
	}

	outBuf, err := proto.Marshal(cgr)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stdout.Write(outBuf); err != nil {
		panic(err)
	}
}

func defaultHybridFromPrefix(prefix string) bool {
	switch prefix {
	// these files should default to the hybrid codegen because code referencing
	// them hasn't been updated yet
	case
		"api/client/proto/event",
		"api/client/proto/inventory",
		"api/client/proto/requestable_roles",
		"api/gen/proto/go/teleport/accessgraph/v1/authorized_key",
		"api/gen/proto/go/teleport/accessgraph/v1/private_key",
		"api/gen/proto/go/teleport/accessgraph/v1/secrets_service",
		"api/gen/proto/go/teleport/accesslist/v1/accesslist_service",
		"api/gen/proto/go/teleport/accesslist/v1/accesslist",
		"api/gen/proto/go/teleport/accessmonitoringrules/v1/access_monitoring_rules_service",
		"api/gen/proto/go/teleport/accessmonitoringrules/v1/access_monitoring_rules",
		"api/gen/proto/go/teleport/appauthconfig/v1/appauthconfig_service",
		"api/gen/proto/go/teleport/appauthconfig/v1/appauthconfig_sessions_service",
		"api/gen/proto/go/teleport/appauthconfig/v1/appauthconfig",
		"api/gen/proto/go/teleport/auditlog/v1/auditlog",
		"api/gen/proto/go/teleport/autoupdate/v1/autoupdate_service",
		"api/gen/proto/go/teleport/autoupdate/v1/autoupdate",
		"api/gen/proto/go/teleport/backendinfo/v1/backendinfo",
		"api/gen/proto/go/teleport/beams/v1/beam_service",
		"api/gen/proto/go/teleport/beams/v1/beam",
		"api/gen/proto/go/teleport/clientiprestriction/v1/clientiprestriction_service",
		"api/gen/proto/go/teleport/clientiprestriction/v1/clientiprestriction",
		"api/gen/proto/go/teleport/clusterconfig/v1/access_graph_settings",
		"api/gen/proto/go/teleport/clusterconfig/v1/access_graph",
		"api/gen/proto/go/teleport/clusterconfig/v1/clusterconfig_service",
		"api/gen/proto/go/teleport/crownjewel/v1/crownjewel_service",
		"api/gen/proto/go/teleport/crownjewel/v1/crownjewel",
		"api/gen/proto/go/teleport/dbobject/v1/dbobject_service",
		"api/gen/proto/go/teleport/dbobject/v1/dbobject",
		"api/gen/proto/go/teleport/dbobjectimportrule/v1/dbobjectimportrule_service",
		"api/gen/proto/go/teleport/dbobjectimportrule/v1/dbobjectimportrule",
		"api/gen/proto/go/teleport/decision/v1alpha1/database_access",
		"api/gen/proto/go/teleport/decision/v1alpha1/decision_service",
		"api/gen/proto/go/teleport/decision/v1alpha1/denial_metadata",
		"api/gen/proto/go/teleport/decision/v1alpha1/enforcement_feature",
		"api/gen/proto/go/teleport/decision/v1alpha1/permit_metadata",
		"api/gen/proto/go/teleport/decision/v1alpha1/request_metadata",
		"api/gen/proto/go/teleport/decision/v1alpha1/resource",
		"api/gen/proto/go/teleport/decision/v1alpha1/ssh_access",
		"api/gen/proto/go/teleport/decision/v1alpha1/ssh_identity",
		"api/gen/proto/go/teleport/decision/v1alpha1/ssh_join",
		"api/gen/proto/go/teleport/decision/v1alpha1/tls_identity",
		"api/gen/proto/go/teleport/delegation/v1/delegation_session_resource",
		"api/gen/proto/go/teleport/delegation/v1/delegation_session_service",
		"api/gen/proto/go/teleport/desktop/v1/tdpb",
		"api/gen/proto/go/teleport/devicetrust/v1/assert",
		"api/gen/proto/go/teleport/devicetrust/v1/authenticate_challenge",
		"api/gen/proto/go/teleport/devicetrust/v1/device_collected_data",
		"api/gen/proto/go/teleport/devicetrust/v1/device_confirmation_token",
		"api/gen/proto/go/teleport/devicetrust/v1/device_enroll_token",
		"api/gen/proto/go/teleport/devicetrust/v1/device_profile",
		"api/gen/proto/go/teleport/devicetrust/v1/device_source",
		"api/gen/proto/go/teleport/devicetrust/v1/device_web_token",
		"api/gen/proto/go/teleport/devicetrust/v1/device",
		"api/gen/proto/go/teleport/devicetrust/v1/devicetrust_service",
		"api/gen/proto/go/teleport/devicetrust/v1/os_type",
		"api/gen/proto/go/teleport/devicetrust/v1/tpm",
		"api/gen/proto/go/teleport/devicetrust/v1/user_certificates",
		"api/gen/proto/go/teleport/discoveryconfig/v1/discoveryconfig_service",
		"api/gen/proto/go/teleport/discoveryconfig/v1/discoveryconfig",
		"api/gen/proto/go/teleport/dynamicwindows/v1/dynamicwindows_service",
		"api/gen/proto/go/teleport/embedding/v1/embedding",
		"api/gen/proto/go/teleport/externalauditstorage/v1/externalauditstorage_service",
		"api/gen/proto/go/teleport/externalauditstorage/v1/externalauditstorage",
		"api/gen/proto/go/teleport/gitserver/v1/git_server_service",
		"api/gen/proto/go/teleport/grpcclientconfig/v1/grpcclientconfig",
		"api/gen/proto/go/teleport/grpcclientconfig/v1/grpcclientconfigservice",
		"api/gen/proto/go/teleport/hardwarekeyagent/v1/hardwarekeyagent_service",
		"api/gen/proto/go/teleport/header/v1/metadata",
		"api/gen/proto/go/teleport/header/v1/resourceheader",
		"api/gen/proto/go/teleport/healthcheckconfig/v1/health_check_config_service",
		"api/gen/proto/go/teleport/healthcheckconfig/v1/health_check_config",
		"api/gen/proto/go/teleport/identitycenter/v1/identitycenter",
		"api/gen/proto/go/teleport/identitycenter/v1/service",
		"api/gen/proto/go/teleport/integration/v1/awsoidc_service",
		"api/gen/proto/go/teleport/integration/v1/awsra_service",
		"api/gen/proto/go/teleport/integration/v1/integration_service",
		"api/gen/proto/go/teleport/inventory/v1/inventory_service",
		"api/gen/proto/go/teleport/issuance/v1/service",
		"api/gen/proto/go/teleport/join/v1/joinservice",
		"api/gen/proto/go/teleport/kube/v1/kube_service",
		"api/gen/proto/go/teleport/kubewaitingcontainer/v1/kubewaitingcontainer_service",
		"api/gen/proto/go/teleport/kubewaitingcontainer/v1/kubewaitingcontainer",
		"api/gen/proto/go/teleport/label/v1/label",
		"api/gen/proto/go/teleport/linuxdesktop/v1/linux_desktop_service",
		"api/gen/proto/go/teleport/linuxdesktop/v1/linux_desktop",
		"api/gen/proto/go/teleport/loginrule/v1/loginrule_service",
		"api/gen/proto/go/teleport/loginrule/v1/loginrule",
		"api/gen/proto/go/teleport/machineid/v1/bot_instance_service",
		"api/gen/proto/go/teleport/machineid/v1/bot_instance",
		"api/gen/proto/go/teleport/machineid/v1/bot_service",
		"api/gen/proto/go/teleport/machineid/v1/bot",
		"api/gen/proto/go/teleport/machineid/v1/federation_service",
		"api/gen/proto/go/teleport/machineid/v1/federation",
		"api/gen/proto/go/teleport/machineid/v1/workload_identity_service",
		"api/gen/proto/go/teleport/notifications/v1/notifications_service",
		"api/gen/proto/go/teleport/notifications/v1/notifications",
		"api/gen/proto/go/teleport/okta/v1/okta_service",
		"api/gen/proto/go/teleport/plugins/v1/plugin_service",
		"api/gen/proto/go/teleport/presence/v1/relay_server",
		"api/gen/proto/go/teleport/presence/v1/service",
		"api/gen/proto/go/teleport/provisioning/v1/provisioning",
		"api/gen/proto/go/teleport/recordingencryption/v1/recording_encryption_service",
		"api/gen/proto/go/teleport/recordingencryption/v1/recording_encryption",
		"api/gen/proto/go/teleport/recordingmetadata/v1/recordingmetadata_service",
		"api/gen/proto/go/teleport/recordingmetadata/v1/recordingmetadata",
		"api/gen/proto/go/teleport/resourceusage/v1/access_requests",
		"api/gen/proto/go/teleport/resourceusage/v1/account_usage_type",
		"api/gen/proto/go/teleport/resourceusage/v1/resourceusage_service",
		"api/gen/proto/go/teleport/samlidp/v1/samlidp",
		"api/gen/proto/go/teleport/scim/v1/scim_service",
		"api/gen/proto/go/teleport/scopes/access/v1/assignment",
		"api/gen/proto/go/teleport/scopes/access/v1/role",
		"api/gen/proto/go/teleport/scopes/access/v1/service",
		"api/gen/proto/go/teleport/scopes/joining/v1/service",
		"api/gen/proto/go/teleport/scopes/joining/v1/token",
		"api/gen/proto/go/teleport/scopes/v1/scopes",
		"api/gen/proto/go/teleport/secreports/v1/secreports_service",
		"api/gen/proto/go/teleport/secreports/v1/secreports",
		"api/gen/proto/go/teleport/sessionsearch/v1/session_search",
		"api/gen/proto/go/teleport/ssh/v1/ssh",
		"api/gen/proto/go/teleport/stableunixusers/v1/stableunixusers",
		"api/gen/proto/go/teleport/subca/v1/cert_authority_override_id",
		"api/gen/proto/go/teleport/subca/v1/cert_authority_override",
		"api/gen/proto/go/teleport/subca/v1/certificate_override_id",
		"api/gen/proto/go/teleport/subca/v1/certificate_override",
		"api/gen/proto/go/teleport/subca/v1/crl",
		"api/gen/proto/go/teleport/subca/v1/csr",
		"api/gen/proto/go/teleport/subca/v1/distinguished_name",
		"api/gen/proto/go/teleport/subca/v1/public_key_hash",
		"api/gen/proto/go/teleport/subca/v1/subca_service",
		"api/gen/proto/go/teleport/summarizer/v1/access_request",
		"api/gen/proto/go/teleport/summarizer/v1/summarizer_service",
		"api/gen/proto/go/teleport/summarizer/v1/summarizer",
		"api/gen/proto/go/teleport/trait/v1/trait",
		"api/gen/proto/go/teleport/transport/v1/transport_service",
		"api/gen/proto/go/teleport/trust/v1/trust_service",
		"api/gen/proto/go/teleport/userloginstate/v1/userloginstate_service",
		"api/gen/proto/go/teleport/userloginstate/v1/userloginstate",
		"api/gen/proto/go/teleport/userprovisioning/v2/statichostuser_service",
		"api/gen/proto/go/teleport/userprovisioning/v2/statichostuser",
		"api/gen/proto/go/teleport/users/v1/users_service",
		"api/gen/proto/go/teleport/usertasks/v1/user_tasks_service",
		"api/gen/proto/go/teleport/usertasks/v1/user_tasks",
		"api/gen/proto/go/teleport/vnet/v1/vnet_config_service",
		"api/gen/proto/go/teleport/vnet/v1/vnet_config",
		"api/gen/proto/go/teleport/workloadcluster/v1/workloadcluster_service",
		"api/gen/proto/go/teleport/workloadcluster/v1/workloadcluster",
		"api/gen/proto/go/teleport/workloadidentity/v1/attrs",
		"api/gen/proto/go/teleport/workloadidentity/v1/issuance_service",
		"api/gen/proto/go/teleport/workloadidentity/v1/join_attrs",
		"api/gen/proto/go/teleport/workloadidentity/v1/resource_service",
		"api/gen/proto/go/teleport/workloadidentity/v1/resource",
		"api/gen/proto/go/teleport/workloadidentity/v1/revocation_resource",
		"api/gen/proto/go/teleport/workloadidentity/v1/revocation_service",
		"api/gen/proto/go/teleport/workloadidentity/v1/sigstore_policy_resource",
		"api/gen/proto/go/teleport/workloadidentity/v1/sigstore_policy_service",
		"api/gen/proto/go/teleport/workloadidentity/v1/sigstore",
		"api/gen/proto/go/teleport/workloadidentity/v1/x509_overrides_service",
		"api/gen/proto/go/teleport/workloadidentity/v1/x509_overrides",
		"api/gen/proto/go/userpreferences/v1/access_graph",
		"api/gen/proto/go/userpreferences/v1/assist",
		"api/gen/proto/go/userpreferences/v1/cluster_preferences",
		"api/gen/proto/go/userpreferences/v1/discover_resource_preferences",
		"api/gen/proto/go/userpreferences/v1/onboard",
		"api/gen/proto/go/userpreferences/v1/sidenav_preferences",
		"api/gen/proto/go/userpreferences/v1/theme",
		"api/gen/proto/go/userpreferences/v1/unified_resource_preferences",
		"api/gen/proto/go/userpreferences/v1/userpreferences",
		"gen/proto/go/accessgraph/v1/session_search",
		"gen/proto/go/accessgraph/v1alpha/access_graph_service",
		"gen/proto/go/accessgraph/v1alpha/aws",
		"gen/proto/go/accessgraph/v1alpha/azure",
		"gen/proto/go/accessgraph/v1alpha/entra",
		"gen/proto/go/accessgraph/v1alpha/events",
		"gen/proto/go/accessgraph/v1alpha/github",
		"gen/proto/go/accessgraph/v1alpha/gitlab",
		"gen/proto/go/accessgraph/v1alpha/graph",
		"gen/proto/go/accessgraph/v1alpha/netiq",
		"gen/proto/go/accessgraph/v1alpha/okta",
		"gen/proto/go/accessgraph/v1alpha/resources",
		"gen/proto/go/teleport/lib/teleterm/auto_update/v1/auto_update_service",
		"gen/proto/go/teleport/lib/teleterm/v1/access_request",
		"gen/proto/go/teleport/lib/teleterm/v1/app",
		"gen/proto/go/teleport/lib/teleterm/v1/auth_settings",
		"gen/proto/go/teleport/lib/teleterm/v1/cluster",
		"gen/proto/go/teleport/lib/teleterm/v1/database",
		"gen/proto/go/teleport/lib/teleterm/v1/gateway",
		"gen/proto/go/teleport/lib/teleterm/v1/kube",
		"gen/proto/go/teleport/lib/teleterm/v1/label",
		"gen/proto/go/teleport/lib/teleterm/v1/server",
		"gen/proto/go/teleport/lib/teleterm/v1/service",
		"gen/proto/go/teleport/lib/teleterm/v1/target_health",
		"gen/proto/go/teleport/lib/teleterm/v1/tshd_events_service",
		"gen/proto/go/teleport/lib/teleterm/v1/usage_events",
		"gen/proto/go/teleport/lib/teleterm/v1/windows_desktop",
		"gen/proto/go/teleport/lib/teleterm/vnet/v1/vnet_service",
		"gen/proto/go/teleport/lib/vnet/diag/v1/diag",
		"gen/proto/go/teleport/lib/vnet/v1/client_application_service",
		"gen/proto/go/teleport/quicpeering/v1alpha/dial",
		"gen/proto/go/teleport/relaypeering/v1alpha/dial",
		"gen/proto/go/teleport/relaytunnel/v1alpha/control",
		"gen/proto/go/teleport/relaytunnel/v1alpha/dial",
		"gen/proto/go/teleport/relaytunnel/v1alpha/discovery_service",
		"gen/proto/go/teleport/storage/local/stableunixusers/v1/stableunixusers",
		"lib/multiplexer/test/ping":
		return true
	default:
		return false
	}
}
