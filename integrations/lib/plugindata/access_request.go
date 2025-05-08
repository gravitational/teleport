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

package plugindata

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/trace"
)

// ResolutionTag represents enum type of access request resolution constant
type ResolutionTag string

const (
	Unresolved       = ResolutionTag("")
	ResolvedApproved = ResolutionTag("APPROVED")
	ResolvedDenied   = ResolutionTag("DENIED")
	ResolvedExpired  = ResolutionTag("EXPIRED")
	ResolvedPromoted = ResolutionTag("PROMOTED")
)

// AccessRequestData represents generic plugin data required for access request processing
type AccessRequestData struct {
	User               string
	Roles              []string
	RequestReason      string
	ReviewsCount       int
	ResolutionTag      ResolutionTag
	ResolutionReason   string
	SystemAnnotations  map[string][]string
	Resources          []string
	SuggestedReviewers []string
	LoginsByRole       map[string][]string
	MaxDuration        *time.Time
}

// DecodeAccessRequestData deserializes a string map to PluginData struct.
func DecodeAccessRequestData(dataMap map[string]string) (data AccessRequestData, err error) {
	data.User = dataMap["user"]
	if str := dataMap["roles"]; str != "" {
		data.Roles = strings.Split(str, ",")
	}
	data.RequestReason = dataMap["request_reason"]
	if str := dataMap["reviews_count"]; str != "" {
		fmt.Sscanf(str, "%d", &data.ReviewsCount)
	}
	data.ResolutionTag = ResolutionTag(dataMap["resolution"])
	data.ResolutionReason = dataMap["resolve_reason"]
	if str := dataMap["max_duration"]; str != "" {
		var maxDuration time.Time
		maxDuration, err = time.Parse(time.RFC3339, str)
		if err != nil {
			err = trace.Wrap(err)
			return
		}
		data.MaxDuration = &maxDuration
	}

	if str, ok := dataMap["resources"]; ok {
		err = json.Unmarshal([]byte(str), &data.Resources)
		if err != nil {
			err = trace.Wrap(err)
			return
		}
	}

	if str, ok := dataMap["system_annotations"]; ok {
		err = json.Unmarshal([]byte(str), &data.SystemAnnotations)
		if err != nil {
			err = trace.Wrap(err)
			return
		}
		if len(data.SystemAnnotations) == 0 {
			data.SystemAnnotations = nil
		}
	}

	if str, ok := dataMap["suggested_reviewers"]; ok {
		err = json.Unmarshal([]byte(str), &data.SuggestedReviewers)
		if err != nil {
			err = trace.Wrap(err)
			return
		}
		if len(data.SuggestedReviewers) == 0 {
			data.SuggestedReviewers = nil
		}
	}

	if str, ok := dataMap["logins_by_role"]; ok {
		err = json.Unmarshal([]byte(str), &data.LoginsByRole)
		if err != nil {
			err = trace.Wrap(err)
			return
		}
		if len(data.LoginsByRole) == 0 {
			data.LoginsByRole = nil
		}
	}
	return
}

// EncodeAccessRequestData deserializes a string map to PluginData struct.
func EncodeAccessRequestData(data AccessRequestData) (map[string]string, error) {
	result := make(map[string]string)

	result["user"] = data.User
	result["roles"] = strings.Join(data.Roles, ",")
	result["resources"] = strings.Join(data.Resources, ",")
	result["request_reason"] = data.RequestReason

	if len(data.Resources) != 0 {
		resources, err := json.Marshal(data.Resources)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["resources"] = string(resources)
	}

	var reviewsCountStr string
	if data.ReviewsCount > 0 {
		reviewsCountStr = fmt.Sprintf("%d", data.ReviewsCount)
	}
	result["reviews_count"] = reviewsCountStr
	result["resolution"] = string(data.ResolutionTag)
	result["resolve_reason"] = data.ResolutionReason
	if data.MaxDuration != nil {
		result["max_duration"] = data.MaxDuration.Format(time.RFC3339)
	}

	if len(data.SystemAnnotations) != 0 {
		annotaions, err := json.Marshal(data.SystemAnnotations)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["system_annotations"] = string(annotaions)
	}

	if len(data.SuggestedReviewers) != 0 {
		reviewers, err := json.Marshal(data.SuggestedReviewers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["suggested_reviewers"] = string(reviewers)
	}

	if len(data.LoginsByRole) != 0 {
		logins, err := json.Marshal(data.LoginsByRole)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result["logins_by_role"] = string(logins)
	}
	return result, nil
}
