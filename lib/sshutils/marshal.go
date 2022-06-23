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

package sshutils

import (
	"fmt"
	"net/url"
	"strings"
)

// MarshalAuthorizedKeysFormat returns the certificate authority public key exported as a single
// line that can be placed in ~/.ssh/authorized_keys file. The format adheres to the
// man sshd (8) authorized_keys format, a space-separated list of: options, keytype,
// base64-encoded key, comment.
// For example:
//
//    cert-authority AAA... type=user&clustername=cluster-a
//
// URL encoding is used to pass the CA type and cluster name into the comment field.
func MarshalAuthorizedKeysFormat(clusterName string, keyBytes []byte) (string, error) {
	comment := url.Values{
		"type":        []string{"user"},
		"clustername": []string{clusterName},
	}

	return fmt.Sprintf("cert-authority %s %s", strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
}

// MarshalAuthorizedHostsFormat returns the certificate authority public key exported as a single line
// that can be placed in ~/.ssh/authorized_hosts. The format adheres to the man sshd (8)
// authorized_hosts format, a space-separated list of: marker, hosts, key, and comment.
// For example:
//
//    @cert-authority *.cluster-a,cluster-a ssh-rsa AAA... type=host
//
// URL encoding is used to pass the CA type and allowed logins into the comment field.
func MarshalAuthorizedHostsFormat(clusterName string, keyBytes []byte, logins []string) (string, error) {
	comment := url.Values{
		"type":   []string{"host"},
		"logins": logins,
	}

	return fmt.Sprintf("@cert-authority %s,*.%s %s %s",
		clusterName, clusterName, strings.TrimSpace(string(keyBytes)), comment.Encode()), nil
}
