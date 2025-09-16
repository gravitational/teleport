/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package config

const (
	// TemplateSSHClientName is the config name for generating ssh client
	// config files.
	TemplateSSHClientName = "ssh_client"

	// TemplateIdentityName is the config name for Teleport identity files.
	TemplateIdentityName = "identity"

	// TemplateTLSName is the config name for TLS client certificates.
	TemplateTLSName = "tls"

	// TemplateTLSCAsName is the config name for TLS CA certificates.
	TemplateTLSCAsName = "tls_cas"

	// TemplateMongoName is the config name for MongoDB-formatted certificates.
	TemplateMongoName = "mongo"

	// TemplateCockroachName is the config name for CockroachDB-formatted
	// certificates.
	TemplateCockroachName = "cockroach"

	// TemplateKubernetesName is the config name for generating Kubernetes
	// client config files
	TemplateKubernetesName = "kubernetes"

	// TemplateSSHHostCertName is the config name for generating SSH host
	// certificates
	TemplateSSHHostCertName = "ssh_host_cert"
)

// FileDescription is a minimal spec needed to create an empty end-user-owned
// file with bot-writable ACLs during `tbot init`.
type FileDescription struct {
	// Name is the name of the file or directory to create.
	Name string

	// IsDir designates whether this describes a subdirectory inside the
	// Destination.
	IsDir bool
}
