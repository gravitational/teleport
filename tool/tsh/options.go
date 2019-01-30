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
	"strings"

	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
)

// AllOptions is a listing of all known OpenSSH options.
var AllOptions = map[string]map[string]bool{
	"AddKeysToAgent":                   map[string]bool{"yes": true},
	"AddressFamily":                    map[string]bool{},
	"BatchMode":                        map[string]bool{},
	"BindAddress":                      map[string]bool{},
	"CanonicalDomains":                 map[string]bool{},
	"CanonicalizeFallbackLocal":        map[string]bool{},
	"CanonicalizeHostname":             map[string]bool{},
	"CanonicalizeMaxDots":              map[string]bool{},
	"CanonicalizePermittedCNAMEs":      map[string]bool{},
	"CertificateFile":                  map[string]bool{},
	"ChallengeResponseAuthentication":  map[string]bool{},
	"CheckHostIP":                      map[string]bool{},
	"Cipher":                           map[string]bool{},
	"Ciphers":                          map[string]bool{},
	"ClearAllForwardings":              map[string]bool{},
	"Compression":                      map[string]bool{},
	"CompressionLevel":                 map[string]bool{},
	"ConnectionAttempts":               map[string]bool{},
	"ConnectTimeout":                   map[string]bool{},
	"ControlMaster":                    map[string]bool{},
	"ControlPath":                      map[string]bool{},
	"ControlPersist":                   map[string]bool{},
	"DynamicForward":                   map[string]bool{},
	"EscapeChar":                       map[string]bool{},
	"ExitOnForwardFailure":             map[string]bool{},
	"FingerprintHash":                  map[string]bool{},
	"ForwardAgent":                     map[string]bool{"yes": true, "no": true},
	"ForwardX11":                       map[string]bool{},
	"ForwardX11Timeout":                map[string]bool{},
	"ForwardX11Trusted":                map[string]bool{},
	"GatewayPorts":                     map[string]bool{},
	"GlobalKnownHostsFile":             map[string]bool{},
	"GSSAPIAuthentication":             map[string]bool{},
	"GSSAPIDelegateCredentials":        map[string]bool{},
	"HashKnownHosts":                   map[string]bool{},
	"Host":                             map[string]bool{},
	"HostbasedAuthentication":          map[string]bool{},
	"HostbasedKeyTypes":                map[string]bool{},
	"HostKeyAlgorithms":                map[string]bool{},
	"HostKeyAlias":                     map[string]bool{},
	"HostName":                         map[string]bool{},
	"IdentityFile":                     map[string]bool{},
	"IdentitiesOnly":                   map[string]bool{},
	"IPQoS":                            map[string]bool{},
	"KbdInteractiveAuthentication":     map[string]bool{},
	"KbdInteractiveDevices":            map[string]bool{},
	"KexAlgorithms":                    map[string]bool{},
	"LocalCommand":                     map[string]bool{},
	"LocalForward":                     map[string]bool{},
	"LogLevel":                         map[string]bool{},
	"MACs":                             map[string]bool{},
	"Match":                            map[string]bool{},
	"NoHostAuthenticationForLocalhost": map[string]bool{},
	"NumberOfPasswordPrompts":          map[string]bool{},
	"PasswordAuthentication":           map[string]bool{},
	"PermitLocalCommand":               map[string]bool{},
	"PKCS11Provider":                   map[string]bool{},
	"Port":                             map[string]bool{},
	"PreferredAuthentications":         map[string]bool{},
	"Protocol":                         map[string]bool{},
	"ProxyCommand":                     map[string]bool{},
	"ProxyUseFdpass":                   map[string]bool{},
	"PubkeyAcceptedKeyTypes":           map[string]bool{},
	"PubkeyAuthentication":             map[string]bool{},
	"RekeyLimit":                       map[string]bool{},
	"RemoteForward":                    map[string]bool{},
	"RequestTTY":                       map[string]bool{"yes": true, "no": true},
	"RhostsRSAAuthentication":          map[string]bool{},
	"RSAAuthentication":                map[string]bool{},
	"SendEnv":                          map[string]bool{},
	"ServerAliveInterval":              map[string]bool{},
	"ServerAliveCountMax":              map[string]bool{},
	"StreamLocalBindMask":              map[string]bool{},
	"StreamLocalBindUnlink":            map[string]bool{},
	"StrictHostKeyChecking":            map[string]bool{"yes": true, "no": true},
	"TCPKeepAlive":                     map[string]bool{},
	"Tunnel":                           map[string]bool{},
	"TunnelDevice":                     map[string]bool{},
	"UpdateHostKeys":                   map[string]bool{},
	"UsePrivilegedPort":                map[string]bool{},
	"User":                             map[string]bool{},
	"UserKnownHostsFile":               map[string]bool{},
	"VerifyHostKeyDNS":                 map[string]bool{},
	"VisualHostKey":                    map[string]bool{},
	"XAuthLocation":                    map[string]bool{},
}

// Options holds parsed values of OpenSSH options.
type Options struct {
	// AddKeysToAgent specifies whether keys should be automatically added to a
	// running SSH agent. Supported options values are "yes".
	AddKeysToAgent bool

	// ForwardAgent specifies whether the connection to the authentication
	// agent will be forwarded to the remote machine. Supported option values
	// are "yes" and "no".
	ForwardAgent bool

	// RequestTTY specifies whether to request a pseudo-tty for the session.
	// Supported option values are "yes" and "no".
	RequestTTY bool

	// StrictHostKeyChecking is used control if tsh will automatically add host
	// keys to the ~/.tsh/known_hosts file. Supported option values are "yes"
	// and "no".
	StrictHostKeyChecking bool
}

func parseOptions(opts []string) (Options, error) {
	// By default, Teleport prefers strict host key checking and adding keys
	// to system SSH agent.
	options := Options{
		StrictHostKeyChecking: true,
		AddKeysToAgent:        true,
	}

	for _, o := range opts {
		key, value, err := splitOption(o)
		if err != nil {
			return Options{}, trace.Wrap(err)
		}

		supportedValues, ok := AllOptions[key]
		if !ok {
			return Options{}, trace.BadParameter("unsupported option key: %v", key)
		}

		if len(supportedValues) == 0 {
			fmt.Printf("WARNING: Option '%v' is not supported.\n", key)
			continue
		}

		_, ok = supportedValues[value]
		if !ok {
			return Options{}, trace.BadParameter("unsupported option value: %v", value)
		}

		switch key {
		case "AddKeysToAgent":
			options.AddKeysToAgent = utils.AsBool(value)
		case "ForwardAgent":
			options.ForwardAgent = utils.AsBool(value)
		case "RequestTTY":
			options.RequestTTY = utils.AsBool(value)
		case "StrictHostKeyChecking":
			options.StrictHostKeyChecking = utils.AsBool(value)
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
	switch {
	case c == ' ':
		return true
	case c == '=':
		return true
	default:
		return false
	}
}
