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
// and a parser/setter if the option is supported by tsh.
var supportedOptions = map[string]setOption{
	"AddKeysToAgent":                   setAddKeysToAgentOption,
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
	"ForwardAgent":                     setAgentForwardingModeOption,
	"ForwardX11":                       setForwardX11Option,
	"ForwardX11Timeout":                setForwardX11TimeoutOption,
	"ForwardX11Trusted":                setForwardX11TrustedOption,
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
	"RequestTTY":                       setRequestTTYOption,
	"RhostsRSAAuthentication":          nil,
	"RSAAuthentication":                nil,
	"SendEnv":                          nil,
	"ServerAliveInterval":              nil,
	"ServerAliveCountMax":              nil,
	"StreamLocalBindMask":              nil,
	"StreamLocalBindUnlink":            nil,
	"StrictHostKeyChecking":            setStrictHostKeyCheckingOption,
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

type setOption func(*Options, string) error

func setAddKeysToAgentOption(o *Options, val string) error {
	parsedValue, err := parseBoolTrueOption(val)
	if err != nil {
		return trace.Wrap(err)
	}
	o.AddKeysToAgent = parsedValue
	return nil
}

func setAgentForwardingModeOption(o *Options, val string) error {
	switch strings.ToLower(val) {
	case "no":
		o.ForwardAgent = client.ForwardAgentNo
	case "yes":
		o.ForwardAgent = client.ForwardAgentYes
	case "local":
		o.ForwardAgent = client.ForwardAgentLocal
	default:
		return trace.BadParameter("invalid agent forwarding mode: %s", val)
	}
	return nil
}

func setForwardX11Option(o *Options, val string) error {
	parsedValue, err := parseBoolOption(val)
	if err != nil {
		return trace.Wrap(err)
	}
	o.ForwardX11 = parsedValue
	return nil
}

func setForwardX11TimeoutOption(o *Options, val string) error {
	parsedValue, err := parseUintOption(val)
	if err != nil {
		return trace.Wrap(err)
	}
	o.ForwardX11Timeout = parsedValue
	return nil
}

func setForwardX11TrustedOption(o *Options, val string) error {
	parsedValue, err := parseBoolOption(val)
	if err != nil {
		return trace.Wrap(err)
	}
	o.ForwardX11Trusted = parsedValue
	return nil
}

func setRequestTTYOption(o *Options, val string) error {
	parsedValue, err := parseBoolOption(val)
	if err != nil {
		return trace.Wrap(err)
	}
	o.RequestTTY = parsedValue
	return nil
}

func setStrictHostKeyCheckingOption(o *Options, val string) error {
	parsedValue, err := parseBoolOption(val)
	if err != nil {
		return trace.Wrap(err)
	}
	o.StrictHostKeyChecking = parsedValue
	return nil
}

func parseBoolOption(val string) (bool, error) {
	if val != "yes" && val != "no" {
		return false, trace.BadParameter("invalid bool option value: %s", val)
	}
	return utils.AsBool(val), nil
}

func parseBoolTrueOption(val string) (bool, error) {
	if val != "yes" {
		return false, trace.BadParameter("invalid true-only bool option value: %s", val)
	}
	return utils.AsBool(val), nil
}

func parseUintOption(val string) (uint, error) {
	valUint, err := strconv.ParseUint(val, 10, 0)
	if err != nil {
		return 0, trace.BadParameter("invalid uint option value: %s", val)
	}
	return uint(valUint), nil
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

		setOption, ok := supportedOptions[key]
		if !ok {
			return Options{}, trace.BadParameter("unsupported option key: %v", key)
		}

		if setOption == nil {
			fmt.Printf("WARNING: Option '%v' is not supported.\n", key)
			continue
		}

		if err := setOption(&options, value); err != nil {
			return Options{}, trace.BadParameter("unsupported option value %q: %s", value, err)
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
