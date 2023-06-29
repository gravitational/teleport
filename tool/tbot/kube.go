/*
Copyright 2022 Gravitational, Inc.

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
	"bytes"
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

func onKubeCredentialsCommand(cfg *config.BotConfig) error {
	destination, err := tshwrap.GetDestinationDirectory(cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	idData, err := destination.Read(config.IdentityFilePath)
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
