/*
Copyright 2023 Gravitational, Inc.

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

package services

import (
	"context"
	"crypto/tls"

	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// TODO(espadolini): delete this after teleport.e is updated with the new package name

func NewPrehogSubmitter(ctx context.Context, prehogEndpoint string, clientCert *tls.Certificate, caCertPEM []byte) (usagereporter.SubmitFunc, error) {
	return usagereporter.NewPrehogSubmitter(ctx, prehogEndpoint, clientCert, caCertPEM)
}

func NewTeleportUsageReporter(log logrus.FieldLogger, clusterName types.ClusterName, submitter usagereporter.SubmitFunc) (*usagereporter.TeleportUsageReporter, error) {
	return usagereporter.NewTeleportUsageReporter(log, clusterName, submitter)
}
