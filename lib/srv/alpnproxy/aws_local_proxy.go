// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alpnproxy

import (
	"net/http"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/lib/utils/aws"
)

// AWSAccessMiddleware verifies the requests to AWS proxy are properly signed.
type AWSAccessMiddleware struct {
	// AWSCredentials are AWS Credentials used by LocalProxy for request's signature verification.
	AWSCredentials *credentials.Credentials

	Log logrus.FieldLogger
}

var _ LocalProxyHTTPMiddleware = &AWSAccessMiddleware{}

func (m *AWSAccessMiddleware) CheckAndSetDefaults() error {
	if m.Log == nil {
		m.Log = logrus.WithField(trace.Component, "aws_access")
	}

	if m.AWSCredentials == nil {
		return trace.BadParameter("missing AWSCredentials")
	}

	return nil
}

func (m *AWSAccessMiddleware) HandleRequest(rw http.ResponseWriter, req *http.Request) bool {
	if err := aws.VerifyAWSSignature(req, m.AWSCredentials); err != nil {
		m.Log.WithError(err).Error("AWS signature verification failed.")
		rw.WriteHeader(http.StatusForbidden)
		return true
	}
	return false
}
