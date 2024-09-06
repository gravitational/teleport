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

package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"

	"github.com/gravitational/teleport/api/identityfile"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/tshwrap"
	"github.com/gravitational/teleport/lib/tlsca"
)

func getCredentialData(idFile *identityfile.IdentityFile, currentTime time.Time) ([]byte, error) {
	cert, err := tlsca.ParseCertificatePEM(idFile.Certs.TLS)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Indicate slightly earlier expiration to avoid the cert expiring
	// mid-request, if possible.
	expiry := cert.NotAfter
	if expiry.Sub(currentTime) > time.Minute {
		expiry = expiry.Add(-1 * time.Minute)
	}
	resp := &clientauthentication.ExecCredential{
		Status: &clientauthentication.ExecCredentialStatus{
			ExpirationTimestamp:   &metav1.Time{Time: expiry},
			ClientCertificateData: string(idFile.Certs.TLS),
			ClientKeyData:         string(idFile.PrivateKey),
		},
	}
	data, err := runtime.Encode(kubeCodecs.LegacyCodec(kubeGroupVersion), resp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func onKubeCredentialsCommand(ctx context.Context, cfg *config.BotConfig) error {
	destination, err := tshwrap.GetDestinationDirectory(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	idData, err := destination.Read(ctx, config.IdentityFilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	idFile, err := identityfile.Read(bytes.NewBuffer(idData))
	if err != nil {
		return trace.Wrap(err)
	}

	data, err := getCredentialData(idFile, time.Now())
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(string(data))
	return nil
}

// Required magic boilerplate to use the k8s encoder.

var (
	kubeScheme       = runtime.NewScheme()
	kubeCodecs       = serializer.NewCodecFactory(kubeScheme)
	kubeGroupVersion = schema.GroupVersion{
		Group:   "client.authentication.k8s.io",
		Version: "v1beta1",
	}
)

func init() {
	metav1.AddToGroupVersion(kubeScheme, schema.GroupVersion{Version: "v1"})
	clientauthv1beta1.AddToScheme(kubeScheme)
	clientauthentication.AddToScheme(kubeScheme)
}
