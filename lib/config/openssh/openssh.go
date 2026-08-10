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

package openssh

import (
	"io"
	"strings"
	"text/template"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
)

// proxyCommandQuote prepares a string for insertion into the ssh_config
func proxyCommandQuote(s string) string {
	s = `'` + strings.ReplaceAll(s, `'`, `'"'"'`) + `'`
	// escape any percent signs which could trigger the percent expansion
	// for ProxyCommand.
	s = strings.ReplaceAll(s, `%`, `%%`)
	// escape any newlines which could impact the parsing of ssh_config
	s = strings.ReplaceAll(s, "\n", `'"\n"'`)
	return s
}

var sshConfigTemplate = template.Must(template.New("ssh-config").Funcs(template.FuncMap{
	"proxyCommandQuote": proxyCommandQuote,
}).Parse(
	`# Begin generated Teleport configuration for {{ .ProxyHost }} by {{ .AppName }}
{{$dot := . }}
{{- range $clusterName := .ClusterNames }}
# Common flags for all {{ $clusterName }} hosts
Host *.{{ $clusterName }} {{ $dot.ProxyHost }}
    UserKnownHostsFile "{{ $dot.KnownHostsPath }}"
    IdentityFile "{{ $dot.IdentityFilePath }}"
    CertificateFile "{{ $dot.CertificateFilePath }}"
    {{- if ne $dot.Username "" }}
    User "{{ $dot.Username }}"
{{- end }}

# Flags for all {{ $clusterName }} hosts except the proxy
Host *.{{ $clusterName }} !{{ $dot.ProxyHost }}
    Port {{ $dot.Port }}
{{- if eq $dot.AppName "tsh" }}
    ProxyCommand "{{ $dot.ExecutablePath }}" proxy ssh --cluster={{ $clusterName }} --proxy={{ $dot.ProxyHost }}:{{ $dot.ProxyPort }} %r@%h:%p
{{- end }}
{{- if eq $dot.AppName "tbot" }}
{{- if $dot.PureTBotProxyCommand }}
    ProxyCommand {{ proxyCommandQuote $dot.ExecutablePath }} ssh-proxy-command --destination-dir={{ proxyCommandQuote $dot.DestinationDir }} --proxy-server={{ proxyCommandQuote (print $dot.ProxyHost ":" $dot.ProxyPort) }} --cluster={{ proxyCommandQuote $clusterName }} {{ if $dot.TLSRouting }}--tls-routing{{ else }}--no-tls-routing{{ end }} {{ if $dot.ConnectionUpgrade }}--connection-upgrade{{ else }}--no-connection-upgrade{{ end }} {{ if $dot.Resume }}--resume{{ else }}--no-resume{{ end }} --user=%r --host=%h --port=%p
{{- else }}
    ProxyCommand "{{ $dot.ExecutablePath }}" proxy --destination-dir={{ $dot.DestinationDir }} --proxy-server={{ $dot.ProxyHost }}:{{ $dot.ProxyPort }} ssh --cluster={{ $clusterName }}  %r@%h:%p
{{- end }}
{{- end }}
{{- end }}
    {{- if ne $dot.Username "" }}
    User "{{ $dot.Username }}"
{{- end }}

# End generated Teleport configuration
`))

// SSHConfigParameters is a set of SSH related parameters used to generate ~/.ssh/config file.
type SSHConfigParameters struct {
	AppName             SSHConfigApps
	ClusterNames        []string
	KnownHostsPath      string
	IdentityFilePath    string
	CertificateFilePath string
	ProxyHost           string
	ProxyPort           string
	ExecutablePath      string
	Username            string
	DestinationDir      string
	// Port is the node port to use, defaulting to 3022, if not specified by flag
	Port int

	// PureTBotProxyCommand enables the new `ssh-proxy-command` operating mode
	// when generating the ssh_config for tbot.
	PureTBotProxyCommand bool
	ConnectionUpgrade    bool
	TLSRouting           bool
	Insecure             bool
	FIPS                 bool
	Resume               bool
}

type sshTmplParams struct {
	SSHConfigParameters
}

// SSHConfigApps represent apps that support ssh config generation.
type SSHConfigApps string

const (
	TshApp  SSHConfigApps = teleport.ComponentTSH
	TbotApp SSHConfigApps = teleport.ComponentTBot
)

// WriteSSHConfig generates an ssh_config file for OpenSSH clients.
func WriteSSHConfig(w io.Writer, config *SSHConfigParameters) error {
	if config.Port == 0 {
		config.Port = defaults.SSHServerListenPort
	}

	if err := sshConfigTemplate.Execute(w, sshTmplParams{
		SSHConfigParameters: *config,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

var muxedSSHConfigTemplate = template.Must(template.New("muxed-ssh-config").Funcs(template.FuncMap{
	"proxyCommandQuote": proxyCommandQuote,
}).Parse(
	`# Begin generated Teleport configuration by {{ .AppName }} for the ssh-multiplexer service
{{$dot := . }}
{{- range $clusterName := .ClusterNames }}
Host *.{{ $clusterName }}
    Port {{ $dot.Port }}
    UserKnownHostsFile {{ proxyCommandQuote $dot.KnownHostsPath }}
    IdentityFile none
    IdentityAgent {{ proxyCommandQuote $dot.AgentSocketPath }}
    ProxyCommand {{range $v := $dot.ProxyCommand}}{{ proxyCommandQuote $v }} {{end}}{{ proxyCommandQuote $dot.MuxSocketPath }} '%h:%p|{{ $clusterName }}'
    ProxyUseFDPass yes
{{- end }}
# End generated Teleport configuration
`))

// MuxedSSHConfigParameters is a set of SSH related parameters used to generate
// a ssh_config file for the ssh-multiplexer service.
type MuxedSSHConfigParameters struct {
	AppName         SSHConfigApps
	ClusterNames    []string
	KnownHostsPath  string
	ProxyCommand    []string
	MuxSocketPath   string
	AgentSocketPath string
	// Port is the node port to use, defaulting to 3022, if not specified by flag
	Port int
}

type muxedSSHTmplParams struct {
	MuxedSSHConfigParameters
}

// WriteMuxedSSHConfig generates a ssh_config file for the ssh-multiplexer service.
func WriteMuxedSSHConfig(w io.Writer, config *MuxedSSHConfigParameters) error {
	if config.Port == 0 {
		config.Port = defaults.SSHServerListenPort
	}

	if err := muxedSSHConfigTemplate.Execute(w, muxedSSHTmplParams{
		MuxedSSHConfigParameters: *config,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

var clusterSSHConfigTmpl = template.Must(template.New("cluster-ssh-config").Funcs(template.FuncMap{
	"proxyCommandQuote": proxyCommandQuote,
}).Parse(
	`# Cluster-specific ssh_config generated by {{ .AppName }} for cluster '{{ .ClusterName }}' via proxy '{{ .ProxyHost }}:{{ .ProxyPort }}'
UserKnownHostsFile "{{ .KnownHostsPath }}"
IdentityFile "{{ .IdentityFilePath }}"
CertificateFile "{{ .CertificateFilePath }}"
Port {{ .Port }}
ProxyCommand {{ proxyCommandQuote .ExecutablePath }} ssh-proxy-command --destination-dir={{ proxyCommandQuote .DestinationDir }} --proxy-server={{ proxyCommandQuote (print .ProxyHost ":" .ProxyPort) }} --cluster={{ proxyCommandQuote .ClusterName }} {{ if .TLSRouting }}--tls-routing{{ else }}--no-tls-routing{{ end }} {{ if .ConnectionUpgrade }}--connection-upgrade{{ else }}--no-connection-upgrade{{ end }} {{ if .Resume }}--resume{{ else }}--no-resume{{ end }} --user=%r --host=%h --port=%p
`))

// ClusterSSHConfigParameters is the parameter set for GetClusterSSHConfig.
type ClusterSSHConfigParameters struct {
	AppName             SSHConfigApps
	ClusterName         string
	KnownHostsPath      string
	IdentityFilePath    string
	CertificateFilePath string
	ProxyHost           string
	ProxyPort           string
	ExecutablePath      string
	DestinationDir      string
	Port                int
	ConnectionUpgrade   bool
	TLSRouting          bool
	Insecure            bool
	FIPS                bool
	Resume              bool
}

type clusterSSHConfigTmplParams struct {
	ClusterSSHConfigParameters
}

// WriteClusterSSHConfig generate a ssh_config that proxies SSH connections via
// tbot and through to a single Teleport cluster. It performs no matching on
// the hostname.
//
// As it does not use the Host match directive, it is also includable within
// another ssh_config, which allows for more complex and customized
// configurations.
func WriteClusterSSHConfig(sb *strings.Builder, config *ClusterSSHConfigParameters) error {
	if config.Port == 0 {
		config.Port = defaults.SSHServerListenPort
	}

	if err := clusterSSHConfigTmpl.Execute(sb, clusterSSHConfigTmplParams{
		ClusterSSHConfigParameters: *config,
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
