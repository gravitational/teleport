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

package aws

import (
	"cmp"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func updateAssumeRoleDuration(identity *tlsca.Identity, w http.ResponseWriter, req *http.Request, clock clockwork.Clock) error {
	assumeRoleReq, identityTTL, err := checkAssumeRoleDuration(identity, req, clock)
	if err != nil {
		return trace.Wrap(err)
	}

	// Skip non-AssumeRole requests and those already within the TTL.
	if assumeRoleReq == nil || assumeRoleReq.getDuration() <= identityTTL {
		return nil
	}

	// Rewrite the request.
	if err := rewriteAssumeRoleRequest(req, assumeRoleReq, identityTTL); err != nil {
		return trace.Wrap(err)
	}
	w.Header().Add(common.TeleportAPIInfoHeader, fmt.Sprintf("requested DurationSeconds of AssumeRole is lowered to \"%d\" as the Teleport identity will expire at %v", int(identityTTL.Seconds()), identity.Expires))
	return nil
}

func denyLongAssumeRoleDuration(identity *tlsca.Identity, req *http.Request, clock clockwork.Clock) error {
	assumeRoleReq, identityTTL, err := checkAssumeRoleDuration(identity, req, clock)
	if err != nil {
		return trace.Wrap(err)
	}
	if assumeRoleReq == nil || assumeRoleReq.getDuration() <= identityTTL {
		return nil
	}

	return trace.AccessDenied("requested DurationSeconds of AssumeRole is longer than the Teleport identity TTL of %v", identityTTL)
}

func checkAssumeRoleDuration(identity *tlsca.Identity, req *http.Request, clock clockwork.Clock) (*assumeRoleRequestParams, time.Duration, error) {
	assumeRoleReq, err := getAssumeRoleRequest(req)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}
	if assumeRoleReq == nil {
		return nil, 0, nil
	}

	identityTTL := identity.Expires.Sub(clock.Now())
	if identityTTL < assumeRoleMinDuration {
		// TODO write error message in XML so the client can understand.
		return nil, 0, trace.AccessDenied("minimum AWS session duration is %v but Teleport identity expires in %v. Please re-login the app and try again.", assumeRoleMinDuration, identityTTL)
	}
	return assumeRoleReq, identityTTL, nil
}

type assumeRoleRequestParams struct {
	query    url.Values
	postForm url.Values

	actionInQuery    bool
	actionInPostForm bool

	durationInQuery    bool
	durationInPostForm bool
}

// getAssumeRoleRequest extracts AssumeRole query and post form values from
// provided request.
//
// AWS SDK reference:
// https://github.com/aws/aws-sdk-go/blob/main/private/protocol/query/build.go
// https://github.com/aws/aws-sdk-go/blob/main/service/sts/api.go
func getAssumeRoleRequest(req *http.Request) (*assumeRoleRequestParams, error) {
	// http.Request.ParseForm may drain the body. Use a clone.
	clone, err := cloneRequest(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := clone.ParseForm(); err != nil {
		return nil, nil
	}

	query := clone.URL.Query()
	postForm := clone.PostForm
	if postForm == nil {
		postForm = url.Values{}
	}

	assumeRoleReq := &assumeRoleRequestParams{
		query:              query,
		postForm:           postForm,
		actionInQuery:      query.Get(assumeRoleQueryKeyAction) == assumeRoleQueryActionAssumeRole,
		actionInPostForm:   postForm.Get(assumeRoleQueryKeyAction) == assumeRoleQueryActionAssumeRole,
		durationInQuery:    query.Has(assumeRoleQueryKeyDurationSeconds),
		durationInPostForm: postForm.Has(assumeRoleQueryKeyDurationSeconds),
	}
	if !assumeRoleReq.actionInQuery && !assumeRoleReq.actionInPostForm {
		return nil, nil
	}
	return assumeRoleReq, nil
}

func parseAssumeRoleDuration(values url.Values) (time.Duration, bool) {
	if durationSeconds, err := strconv.ParseInt(values.Get(assumeRoleQueryKeyDurationSeconds), 10, 32); err == nil {
		return time.Duration(durationSeconds) * time.Second, true
	}
	return 0, false
}

func (r *assumeRoleRequestParams) getDuration() time.Duration {
	var duration time.Duration
	for _, values := range []url.Values{r.query, r.postForm} {
		if candidate, ok := parseAssumeRoleDuration(values); ok && candidate > duration {
			duration = candidate
		}
	}
	return cmp.Or(duration, assumeRoleDefaultDuration)
}

func rewriteAssumeRoleRequest(req *http.Request, assumeRoleReq *assumeRoleRequestParams, duration time.Duration) error {
	durationSeconds := strconv.Itoa(int(duration.Seconds()))
	if assumeRoleReq.actionInQuery || assumeRoleReq.durationInQuery {
		query := req.URL.Query()
		query.Set(assumeRoleQueryKeyDurationSeconds, durationSeconds)
		req.URL.RawQuery = query.Encode()
		if req.RequestURI != "" {
			req.RequestURI = req.URL.RequestURI()
		}
	}

	if !assumeRoleReq.actionInPostForm && !assumeRoleReq.durationInPostForm {
		return nil
	}

	assumeRoleReq.postForm.Set(assumeRoleQueryKeyDurationSeconds, durationSeconds)
	body := assumeRoleReq.postForm.Encode()
	if err := utils.ReplaceRequestBody(req, io.NopCloser(strings.NewReader(body))); err != nil {
		return trace.Wrap(err)
	}
	req.ContentLength = int64(len(body))
	return nil
}

const (
	// assumeRoleQueryKeyAction is the query key for the STS API action.
	assumeRoleQueryKeyAction = "Action"

	// assumeRoleQueryActionAssumeRole is the STS AssumeRole API action name.
	assumeRoleQueryActionAssumeRole = "AssumeRole"

	// assumeRoleQueryKeyDurationSeconds is the query key for the duration
	// seconds for the AssumeRole request.
	assumeRoleQueryKeyDurationSeconds = "DurationSeconds"

	// assumeRoleMinDuration is the minimum duration that can be set for
	// the AssumeRole request.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	assumeRoleMinDuration = 15 * time.Minute
	// assumeRoleDefaultDuration is the default duration if DurationSeconds is
	// not explicitly set in the AssumeRole request.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
	assumeRoleDefaultDuration = 1 * time.Hour
)
