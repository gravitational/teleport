// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package bedrock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/awsconfig"
)

// SignRequestOptions contains the options used to sign a Bedrock request.
type SignRequestOptions struct {
	// Logger is the logger used to emit log entries.
	Logger *slog.Logger
	// App is the application the request is being made to.
	App types.Application
	// Credentials provides the AWS configuration used to retrieve the
	// credentials that sign the request.
	Credentials awsconfig.Provider
	// Request is the HTTP request to be signed.
	Request *http.Request
	// RequestBody is the raw request body used to compute the payload hash.
	RequestBody []byte
}

// CheckAndSetDefaults validates the options and sets default values.
func (o *SignRequestOptions) CheckAndSetDefaults() error {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.App == nil {
		return trace.BadParameter("app is required")
	}
	if o.Credentials == nil {
		return trace.BadParameter("credentials is required")
	}
	if o.Request == nil {
		return trace.BadParameter("http request is required")
	}
	return nil
}

// SignRequest signs Bedrock requests.
func SignRequest(ctx context.Context, opts SignRequestOptions) error {
	if err := opts.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	region := resolveRegion(ctx, opts.Logger, opts.App)
	hash := sha256.New()
	hash.Write(opts.RequestBody)
	bodyHash := hex.EncodeToString(hash.Sum(nil))

	awsCfg, err := opts.Credentials.GetConfig(ctx, region,
		// When empty this already defaults to using ambient credentials.
		awsconfig.WithCredentialsMaybeIntegration(awsconfig.IntegrationMetadata{
			Name: opts.App.GetIntegration(),
		}),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	creds, err := awsCfg.Credentials.Retrieve(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	signer := v4.NewSigner()
	if err := signer.SignHTTP(ctx, creds, opts.Request, bodyHash, mantleServiceName, region, time.Now()); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// BuildURL builds the Bedrock URL address.
func BuildURL(log *slog.Logger, app types.Application) (*url.URL, error) {
	format := app.GetLLM().Format
	region := resolveRegion(context.Background(), log, app)
	// https://docs.aws.amazon.com/bedrock/latest/userguide/endpoints.html
	host := "bedrock-mantle." + region + ".api.aws"

	switch format {
	case types.LLMFormatAnthropic:
		// Messages API: https://docs.aws.amazon.com/bedrock/latest/userguide/inference-messages-api.html
		return &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/anthropic/v1",
		}, nil
	case types.LLMFormatOpenAI:
		// Responses API: https://docs.aws.amazon.com/bedrock/latest/userguide/bedrock-mantle.html
		// Chat completions API: https://docs.aws.amazon.com/bedrock/latest/userguide/inference-chat-completions-mantle.html
		return &url.URL{
			Scheme: "https",
			Host:   host,
			Path:   "/v1",
		}, nil
	default:
		return nil, trace.BadParameter("format %q not supported on AWS Bedrock", format)
	}
}

// resolveRegion resolves which AWS region to use.
//
// We highly recommend apps to explicitly configure the region, however it
// is currently optional. So, instead of letting a valid app fails 100% of
// the requests (due to missing the AWS region), we set a default region
// and emit a log message.
func resolveRegion(ctx context.Context, log *slog.Logger, app types.Application) string {
	if region := app.GetAWSRegion(); region != "" {
		return region
	}

	log.InfoContext(ctx, "app doesn't specify a region, using default value", "region", defaultRegion)
	return defaultRegion
}

const (
	// mantleServiceName is the bedrock mantle IAM service name used to sign
	// requests.
	mantleServiceName = "bedrock-mantle"

	// defaultRegion is the bedrock mantle region used if none was provided.
	defaultRegion = "us-east-1"
)
