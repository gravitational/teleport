package secrets

import (
	"context"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

func Fetch(ctx context.Context, resourceName string) ([]byte, error) {
	c, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed creating SecretManager client")
	}
	defer c.Close()

	log.Debugf("Fetching secret %s", resourceName)
	secret, err := c.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: resourceName,
	})

	if err != nil {
		return nil, trace.Wrap(err, "failed fetching secret token")
	}

	return secret.Payload.Data, nil
}

func FetchString(ctx context.Context, resourceName string) (string, error) {
	data, err := Fetch(ctx, resourceName)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(data), nil
}
