/*
Copyright 2020-2021 Gravitational, Inc.

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
