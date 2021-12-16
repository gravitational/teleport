/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// supportedOptions is a listing of all known OpenSSH options
// and a validater/parser if the option is supported by tsh.
var supportedOptions = map[string]validateAndParseValue{
	"AddKeysToAgent":                   parseBoolTrueOption,
	"AddressFamily":                    nil,
	"BatchMode":                        nil,
	"BindAddress":                      nil,
	"CanonicalDomains":                 nil,
	"CanonicalizeFallbackLocal":        nil,
	"CanonicalizeHostname":             nil,
	"CanonicalizeMaxDots":              nil,
	"CanonicalizePermittedCNAMEs":      nil,
	"CertificateFile":                  nil,
	"ChallengeResponseAuthentication":  nil,
	"CheckHostIP":                      nil,
	"Cipher":                           nil,
	"Ciphers":                          nil,
	"ClearAllForwardings":              nil,
	"Compression":                      nil,
	"CompressionLevel":                 nil,
	"ConnectionAttempts":               nil,
	"ConnectTimeout":                   nil,
	"ControlMaster":                    nil,
	"ControlPath":                      nil,
	"ControlPersist":                   nil,
	"DynamicForward":                   nil,
	"EscapeChar":                       nil,
	"ExitOnForwardFailure":             nil,
	"FingerprintHash":                  nil,
	"ForwardAgent":                     parseAgentForwardingMode,
	"ForwardX11":                       parseBoolOption,
	"ForwardX11Timeout":                parseUintOption,
	"ForwardX11Trusted":                parseBoolOption,
	"GatewayPorts":                     nil,
	"GlobalKnownHostsFile":             nil,
	"GSSAPIAuthentication":             nil,
	"GSSAPIDelegateCredentials":        nil,
	"HashKnownHosts":                   nil,
	"Host":                             nil,
	"HostbasedAuthentication":          nil,
	"HostbasedKeyTypes":                nil,
	"HostKeyAlgorithms":                nil,
	"HostKeyAlias":                     nil,
	"HostName":                         nil,
	"IdentityFile":                     nil,
	"IdentitiesOnly":                   nil,
	"IPQoS":                            nil,
	"KbdInteractiveAuthentication":     nil,
	"KbdInteractiveDevices":            nil,
	"KexAlgorithms":                    nil,
	"LocalCommand":                     nil,
	"LocalForward":                     nil,
	"LogLevel":                         nil,
	"MACs":                             nil,
	"Match":                            nil,
	"NoHostAuthenticationForLocalhost": nil,
	"NumberOfPasswordPrompts":          nil,
	"PasswordAuthentication":           nil,
	"PermitLocalCommand":               nil,
	"PKCS11Provider":                   nil,
	"Port":                             nil,
	"PreferredAuthentications":         nil,
	"Protocol":                         nil,
	"ProxyCommand":                     nil,
	"ProxyUseFdpass":                   nil,
	"PubkeyAcceptedKeyTypes":           nil,
	"PubkeyAuthentication":             nil,
	"RekeyLimit":                       nil,
	"RemoteForward":                    nil,
	"RequestTTY":                       parseBoolOption,
	"RhostsRSAAuthentication":          nil,
	"RSAAuthentication":                nil,
	"SendEnv":                          nil,
	"ServerAliveInterval":              nil,
	"ServerAliveCountMax":              nil,
	"StreamLocalBindMask":              nil,
	"StreamLocalBindUnlink":            nil,
	"StrictHostKeyChecking":            parseBoolOption,
	"TCPKeepAlive":                     nil,
	"Tunnel":                           nil,
	"TunnelDevice":                     nil,
	"UpdateHostKeys":                   nil,
	"UsePrivilegedPort":                nil,
	"User":                             nil,
	"UserKnownHostsFile":               nil,
	"VerifyHostKeyDNS":                 nil,
	"VisualHostKey":                    nil,
	"XAuthLocation":                    nil,
}

type validateAndParseValue func(string) (interface{}, error)

func parseBoolOption(val string) (interface{}, error) {
	if val != "yes" && val != "no" {
		return nil, trace.BadParameter("invalid bool option value: %s", val)
	}
	return utils.AsBool(val), nil
}

func parseBoolTrueOption(val string) (interface{}, error) {
	if val != "yes" {
		return nil, trace.BadParameter("invalid true-only bool option value: %s", val)
	}
	return utils.AsBool(val), nil
}

func parseUintOption(val string) (interface{}, error) {
	valUint, err := strconv.ParseUint(val, 10, 0)
	if err != nil {
		return Options{}, trace.BadParameter("invalid int option value: %s", val)
	}
	return uint(valUint), nil
}

func parseAgentForwardingMode(val string) (interface{}, error) {
	switch strings.ToLower(val) {
	case "no":
		return client.ForwardAgentNo, nil

	case "yes":
		return client.ForwardAgentYes, nil

	case "local":
		return client.ForwardAgentLocal, nil

	default:
		return Options{}, trace.BadParameter("invalid agent forwarding mode: %s", val)

	}
}

// Options holds parsed values of OpenSSH options.
type Options struct {
	// AddKeysToAgent specifies whether keys should be automatically added to a
	// running SSH agent. Supported options values are "yes".
	AddKeysToAgent bool

	// ForwardAgent specifies whether the connection to the authentication
	// agent will be forwarded to the remote machine. Supported option values
	// are "yes", "no", and "local".
	ForwardAgent client.AgentForwardingMode

	// RequestTTY specifies whether to request a pseudo-tty for the session.
	// Supported option values are "yes" and "no".
	RequestTTY bool

	// StrictHostKeyChecking is used control if tsh will automatically add host
	// keys to the ~/.tsh/known_hosts file. Supported option values are "yes"
	// and "no".
	StrictHostKeyChecking bool

	// ForwardX11 specifies whether X11 forwarding should be enabled for
	// ssh sessions started by the client. Supported option values are "yes" and "no".
	ForwardX11 bool

	// ForwardX11Trusted specifies whether X11Forwarding will be carried out in
	// trusted or untrusted mode when enabled. Supported option values are "yes" and "no".
	ForwardX11Trusted bool

	// ForwardX11Timeout specifies a timeout in seconds after which x11 forwarding
	// attempts will become unauthorized. Only available in untrusted x11 forwarding.
	// Supports uint option values.
	ForwardX11Timeout uint
}

func parseOptions(opts []string) (Options, error) {
	options := Options{
		// By default, Teleport prefers strict host key checking and adding keys
		// to system SSH agent.
		StrictHostKeyChecking: true,
		AddKeysToAgent:        true,
		// As in OpenSSH, untrusted mode is the default unless explicitly set to false.
		// Although it makes clients using x11 forwarding vulnerable to some XServer
		// related attacks (such as Keystroke monitoring), most programs do not properly
		// support untrusted mode and will crash. For these reasons, users are encouraged
		// to enable x11 forwarding with caution, as an attacker that can bypass file
		// permissions on the remote host may be able to carry out such attacks.
		ForwardX11Trusted: true,
	}

	for _, o := range opts {
		key, value, err := splitOption(o)
		if err != nil {
			return Options{}, trace.Wrap(err)
		}

		parseValue, ok := supportedOptions[key]
		if !ok {
			return Options{}, trace.BadParameter("unsupported option key: %v", key)
		}

		if parseValue == nil {
			fmt.Printf("WARNING: Option '%v' is not supported.\n", key)
			continue
		}

		val, err := parseValue(value)
		if err != nil {
			return Options{}, trace.BadParameter("unsupported option value %q: %s", value, err)
		}

		switch key {
		case "AddKeysToAgent":
			options.AddKeysToAgent = val.(bool)
		case "ForwardAgent":
			options.ForwardAgent = val.(client.AgentForwardingMode)
		case "RequestTTY":
			options.RequestTTY = val.(bool)
		case "StrictHostKeyChecking":
			options.StrictHostKeyChecking = val.(bool)
		case "ForwardX11":
			options.ForwardX11 = val.(bool)
		case "ForwardX11Trusted":
			options.ForwardX11Trusted = val.(bool)
		case "ForwardX11Timeout":
			options.ForwardX11Timeout = val.(uint)
		}
	}

	return options, nil
}

func splitOption(option string) (string, string, error) {
	parts := strings.FieldsFunc(option, fieldsFunc)

	if len(parts) != 2 {
		return "", "", trace.BadParameter("invalid format for option")
	}

	return parts[0], parts[1], nil
}

// fieldsFunc splits key-value pairs off ' ' and '='.
func fieldsFunc(c rune) bool {
	switch c {
	case ' ', '=':
		return true
	default:
		return false
	}
}
