/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"net/url"
	"strings"

	"github.com/gravitational/teleport/api/constants"
)

const (
	awsRegionalConsoleHostSuffix = ".console.aws.amazon.com"
)

var (
	awsConsoleHost      = mustURLHostname(constants.AWSConsoleURL)
	awsUSGovConsoleHost = mustURLHostname(constants.AWSUSGovConsoleURL)
	awsCNConsoleHost    = mustURLHostname(constants.AWSCNConsoleURL)
	awsQuickSightHost   = mustURLHostname(constants.AWSQuickSightURL)
)

// IsConsoleURL returns true when rawURL is a supported AWS console URL.
func IsConsoleURL(rawURL string) bool {
	_, ok := ConsoleURLPartition(rawURL)
	return ok
}

// ConsoleURLPartition returns the AWS partition for a supported AWS console URL.
func ConsoleURLPartition(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return "", false
	}
	if parsed.User != nil {
		return "", false
	}
	switch parsed.Port() {
	case "", "443":
	default:
		return "", false
	}

	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if partition, ok := regionalConsoleURLPartition(host); ok {
		return partition, true
	}

	switch host {
	case awsConsoleHost, awsQuickSightHost:
		return StandardPartition, true
	case awsUSGovConsoleHost:
		return USGovPartition, true
	case awsCNConsoleHost:
		return CNPartition, true
	default:
		return "", false
	}
}

func regionalConsoleURLPartition(host string) (string, bool) {
	region, ok := strings.CutSuffix(host, awsRegionalConsoleHostSuffix)
	if !ok || region == AWSGlobalRegion || IsValidRegion(region) != nil {
		return "", false
	}
	partition := GetPartitionFromRegion(region)
	if partition != StandardPartition {
		return "", false
	}
	return partition, true
}

func mustURLHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
}
