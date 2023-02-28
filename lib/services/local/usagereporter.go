package local

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

func NewUsageReporter(_ context.Context, log logrus.FieldLogger, clusterName types.ClusterName, submitter usagereporter.SubmitFunc) (*usagereporter.TeleportUsageReporter, error) {
	return usagereporter.NewTeleportUsageReporter(log, clusterName, submitter)
}
