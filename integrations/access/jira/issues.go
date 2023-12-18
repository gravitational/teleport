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

package jira

import (
	"strings"

	"github.com/gravitational/trace"
)

// IssueUpdate is an info of who updated the issue status.
type IssueUpdate struct {
	Status string
	Author UserDetails
}

// GetRequestID extracts a request id from issue properties.
func GetRequestID(issue Issue) (string, error) {
	reqID, ok := issue.Properties[RequestIDPropertyKey].(string)
	if !ok {
		return "", trace.Errorf("got non-string %s field", RequestIDPropertyKey)
	}
	return reqID, nil
}

// GetLastUpdate returns a last issue update by a status name.
func GetLastUpdate(issue Issue, status string) (IssueUpdate, error) {
	changelog := issue.Changelog
	if len(changelog.Histories) == 0 {
		return IssueUpdate{}, trace.Errorf("changelog is missing in API response")
	}

	var update *IssueUpdate
	for _, entry := range changelog.Histories {
		for _, item := range entry.Items {
			if item.FieldType == "jira" && item.Field == "status" && strings.ToLower(item.ToString) == status {
				update = &IssueUpdate{
					Status: status,
					Author: entry.Author,
				}
				break
			}
		}
		if update != nil {
			break
		}
	}
	if update == nil {
		return IssueUpdate{}, trace.Errorf("cannot find a %s status update in changelog", status)
	}
	return *update, nil
}

// GetTransition returns an issue transition by a status name.
func GetTransition(issue Issue, status string) (IssueTransition, error) {
	for _, transition := range issue.Transitions {
		if strings.ToLower(transition.To.Name) == status {
			return transition, nil
		}
	}
	return IssueTransition{}, trace.Errorf("cannot find a %s status among possible transitions", status)
}
