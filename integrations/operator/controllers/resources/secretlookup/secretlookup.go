/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package secretlookup

import (
	"context"
	"net/url"
	"strings"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gravitational/teleport/api/types"
)

const (
	// secretScheme is the URI scheme for Kubernetes Secret Lookup
	secretScheme = "secret"
	secretPrefix = secretScheme + "://"

	// AllowLookupAnnotation is the annotation a secret must wear for the operator to allow looking up its content
	// when reconciling a resource. Its value is either a comma-separated list of allowed resources, or a '*'.
	AllowLookupAnnotation = "resources.teleport.dev/allow-lookup-from-cr"
)

// IsNeeded checks if a string starts with "secret://" and needs a secret lookup.
func IsNeeded(value string) bool {
	return strings.HasPrefix(value, secretPrefix)
}

// Try takes a URI such as "secret://secret-name/secret-key" and returns the data from the secret key.
// To protect against someone abusing this to read from arbitrary secrets, the secret must be annotated with the
// names of the CRs allowed to include it (or a wildcard).
func Try(ctx context.Context, clt kclient.Client, name, namespace, uri string) (string, error) {
	// Parse and check the URI
	u, err := url.Parse(uri)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if u.Scheme != secretScheme {
		return "", trace.BadParameter("invalid secret scheme %q", u.Scheme)
	}

	if u.Host == "" {
		return "", trace.BadParameter("missing secret name")
	}

	if u.Path == "" {
		return "", trace.BadParameter("missing secret key")
	}
	key := strings.TrimPrefix(u.Path, "/")

	// Lookup the secret
	secret := &corev1.Secret{}
	if err := clt.Get(ctx, kclient.ObjectKey{Namespace: namespace, Name: u.Host}, secret); err != nil {
		return "", trace.Wrap(err, "failed to lookup secret")
	}

	// Checking permission
	if err := isInclusionAllowed(secret, name); err != nil {
		return "", trace.Wrap(err)
	}

	secretData, ok := secret.Data[key]
	if !ok {
		return "", trace.BadParameter("secret %q is missing key %q", u.Host, key)
	}

	// Apparently we don't need to b64 decode ¯\_(ツ)_/¯
	return string(secretData), nil
}

// isInclusionAllowed checks if the secret allows inclusion from the CR.
// The secret must wear the AllowLookupAnnotation and the annotation must either
// explicitly allow the resource, or allow any resource ("*").
func isInclusionAllowed(secret *corev1.Secret, name string) error {
	secretName := secret.Name
	annotation, ok := secret.Annotations[AllowLookupAnnotation]
	if !ok {
		return trace.BadParameter("secret %q doesn't have the %q annotation", secretName, AllowLookupAnnotation)
	}
	if annotation == types.Wildcard {
		return nil
	}
	allowedCRs := strings.Split(annotation, ",")
	for _, allowedCR := range allowedCRs {
		if strings.TrimSpace(allowedCR) == name {
			return nil
		}
	}
	return trace.AccessDenied("secret %q have the annotation %q but it does not contain %q", secretName, AllowLookupAnnotation, name)
}
