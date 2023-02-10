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

package aws

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func updateAssumeRoleDuration(identity *tlsca.Identity, req *http.Request, clock clockwork.Clock) error {
	// Skip non-AssumeRole request
	query, err := getAssumeRoleQuery(req)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	requestedDuration := getAssumeRoleDuration(query)
	identityDuration := identity.Expires.Sub(clock.Now())
	switch {
	// Deny access if identity is expiring soon.
	case identityDuration < assumeRoleMinDuration:
		return trace.AccessDenied("minimum AWS session duration is %v but Teleport identity expires in %v", assumeRoleMinDuration, identityDuration)

	// Use shorter identity duration.
	case identityDuration < requestedDuration:
		setAssumeRoleDuration(query, identityDuration)
		newBody := io.NopCloser(strings.NewReader(query.Encode()))
		return trace.Wrap(utils.ReplaceRequestBody(req, newBody))

	// Use shorter requested duration (no update required).
	default:
		return nil
	}
}

// getAssumeRoleQuery extracts AssumeRole query values from provided request.
//
// AWS SDK reference:
// https://github.com/aws/aws-sdk-go/blob/main/private/protocol/query/build.go
// https://github.com/aws/aws-sdk-go/blob/main/service/sts/api.go
func getAssumeRoleQuery(req *http.Request) (url.Values, error) {
	// http.Request.ParseForm may drain the body. Use a clone.
	clone, err := cloneRequest(req)
	if err != nil {
		return nil, trace.Wrap(nil)
	}
	if err := clone.ParseForm(); err != nil {
		return nil, trace.NotFound("request is not a post form")
	}
	if clone.PostForm.Get("Action") != "AssumeRole" {
		return nil, trace.NotFound("query action is not AssumeRole")
	}
	return clone.PostForm, nil
}

func getAssumeRoleDuration(query url.Values) time.Duration {
	if durationSeconds, err := strconv.ParseInt(query.Get(assumeRoleQueryKeyDurationSeconds), 10, 32); err == nil {
		return time.Duration(durationSeconds) * time.Second
	}
	return assumeRoleDefaultDuration
}
func setAssumeRoleDuration(query url.Values, duration time.Duration) {
	query.Set(assumeRoleQueryKeyDurationSeconds, strconv.Itoa(int(duration.Round(time.Second).Seconds())))
}

const (
	// assumeRoleQueryKeyDurationSeconds is the query key for the duration
	// seconds for the AssumeRole request.
	assumeRoleQueryKeyDurationSeconds = "DurationSeconds"

	// assumeRoleMinDuration is the minimum duration that can be set for
	// the AssumeRole request.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	assumeRoleMinDuration = 15 * time.Minute
	// assumeRoleMinDuration is the default duration if DurationSeconds is not
	// explicitly set in the AssumeRole request.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	assumeRoleDefaultDuration = 1 * time.Hour
)
