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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var sampleAccessRequestData = AccessRequestData{
	User:               "user-foo",
	Roles:              []string{"role-foo", "role-bar"},
	Resources:          []string{"cluster/node/foo", "cluster/node/bar"},
	RequestReason:      "foo reason",
	ReviewsCount:       3,
	ResolutionTag:      ResolvedApproved,
	ResolutionReason:   "foo ok",
	SuggestedReviewers: []string{"foouser"},
	LoginsByRole: map[string][]string{
		"role-foo": {"login-foo", "login-bar"},
	},
	RequestTTL:    "2h0m0s",
	RequestExpiry: mustParseTime("2006-01-02T17:04:05Z"),
	AccessTTL:     "8h0m0s",
	AccessExpiry:  mustParseTime("2006-01-02T23:04:05Z"),
}

func mustParseTime(raw string) *time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestEncodeAccessRequestData(t *testing.T) {
	dataMap, err := EncodeAccessRequestData(sampleAccessRequestData)
	assert.NoError(t, err)
	assert.Len(t, dataMap, 13)
	assert.Equal(t, "user-foo", dataMap["user"])
	assert.Equal(t, "role-foo,role-bar", dataMap["roles"])
	assert.Equal(t, `["cluster/node/foo","cluster/node/bar"]`, dataMap["resources"])
	assert.Equal(t, "foo reason", dataMap["request_reason"])
	assert.Equal(t, "3", dataMap["reviews_count"])
	assert.Equal(t, "APPROVED", dataMap["resolution"])
	assert.Equal(t, "foo ok", dataMap["resolve_reason"])
	assert.Equal(t, `["foouser"]`, dataMap["suggested_reviewers"])
	assert.Equal(t, `{"role-foo":["login-foo","login-bar"]}`, dataMap["logins_by_role"])
	assert.Equal(t, "2h0m0s", dataMap["request_ttl"])
	assert.Equal(t, "2006-01-02T17:04:05Z", dataMap["request_expiry"])
	assert.Equal(t, "8h0m0s", dataMap["access_ttl"])
	assert.Equal(t, "2006-01-02T23:04:05Z", dataMap["access_expiry"])

}

func TestDecodeAccessRequestData(t *testing.T) {
	pluginData, err := DecodeAccessRequestData(map[string]string{
		"user":                "user-foo",
		"roles":               "role-foo,role-bar",
		"resources":           `["cluster/node/foo", "cluster/node/bar"]`,
		"request_reason":      "foo reason",
		"reviews_count":       "3",
		"resolution":          "APPROVED",
		"resolve_reason":      "foo ok",
		"suggested_reviewers": `["foouser"]`,
		"logins_by_role":      `{"role-foo":["login-foo","login-bar"]}`,
		"request_ttl":         "2h0m0s",
		"request_expiry":      "2006-01-02T17:04:05Z",
		"access_ttl":          "8h0m0s",
		"access_expiry":       "2006-01-02T23:04:05Z",
	})
	assert.NoError(t, err)
	assert.Equal(t, sampleAccessRequestData, pluginData)
}

func TestEncodeEmptyAccessRequestData(t *testing.T) {
	dataMap, err := EncodeAccessRequestData(AccessRequestData{})
	assert.NoError(t, err)
	assert.Len(t, dataMap, 7)
	for key, value := range dataMap {
		assert.Emptyf(t, value, "value at key %q must be empty", key)
	}
}

func TestDecodeEmptyAccessRequestData(t *testing.T) {
	decoded, err := DecodeAccessRequestData(nil)
	assert.NoError(t, err)
	assert.Empty(t, decoded)
	decoded, err = DecodeAccessRequestData(make(map[string]string))
	assert.NoError(t, err)
	assert.Empty(t, decoded)
}
