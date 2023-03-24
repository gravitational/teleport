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

package okta

import (
	"context"
	"sort"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// buildImportRuleMappings will build the import rule mappings used to determine
// which labels to apply to groups and applications synchronized by this service.
// This is intended to be run once per synchronization, and the mappings will
// then be used for the entirety of the synchronization run.
func (s *Service) buildImportRuleMappings(ctx context.Context) error {
	s.groupIRMappingMu.Lock()
	defer s.groupIRMappingMu.Unlock()
	s.applicationIRMappingMu.Lock()
	defer s.applicationIRMappingMu.Unlock()

	// Clear out the existing caches
	s.groupIRMapping = map[string]prioritizedLabels{}
	s.applicationIRMapping = map[string]prioritizedLabels{}

	var nextToken string
	for {
		var oktaImportRules []types.OktaImportRule
		var err error
		oktaImportRules, nextToken, err = s.accessPoint.ListOktaImportRules(ctx, 0, nextToken)
		if err != nil {
			return trace.Wrap(err)
		}

		// Create a map of group/application IDs to import rules. We won't worry about sorting these
		// for now.
		for _, importRule := range oktaImportRules {
			for _, mapping := range importRule.GetMappings() {
				for _, match := range mapping.GetMatches() {
					p := newPriorityAndLabel(importRule.GetPriority(), mapping.GetAddLabels())
					if ok, groupIDs := match.GetGroupIDs(); ok {
						for _, groupID := range groupIDs {
							s.groupIRMapping[groupID] = append(s.groupIRMapping[groupID], p)
						}
					}
					if ok, appIDs := match.GetAppIDs(); ok {
						for _, appID := range appIDs {
							s.applicationIRMapping[appID] = append(s.applicationIRMapping[appID], p)
						}
					}
				}
			}
		}

		if nextToken == "" {
			break
		}
	}

	// Sort the Okta import rules by priority.
	for _, v := range s.groupIRMapping {
		sort.Sort(v)
	}
	for _, v := range s.applicationIRMapping {
		sort.Sort(v)
	}

	return nil
}

// getGroupLabels will return the group labels for the given group ID.
func (s *Service) getGroupLabels(groupID string) map[string]string {
	s.groupIRMappingMu.RLock()
	defer s.groupIRMappingMu.RUnlock()

	return aggregateLabels(s.groupIRMapping[groupID])
}

// getApplicationLabels will return the application labels for the given application ID.
func (s *Service) getApplicationLabels(applicationID string) map[string]string {
	s.applicationIRMappingMu.RLock()
	defer s.applicationIRMappingMu.RUnlock()

	return aggregateLabels(s.applicationIRMapping[applicationID])
}

// aggregateLabels will return labels applied to a map, applied in order.
func aggregateLabels(p prioritizedLabels) map[string]string {
	labels := map[string]string{}
	for _, priorityAndLabels := range p {
		for name, value := range priorityAndLabels.addLabels {
			labels[name] = value
		}
	}

	return labels
}

// priorityAndLabels is a struct that will store labels to be added to a group or
// application along with its associated priority.
type priorityAndLabels struct {
	priority  int32
	addLabels map[string]string
}

// newPriorityAndLabels creates a new priority and labels object.
func newPriorityAndLabel(priority int32, addLabels map[string]string) priorityAndLabels {
	return priorityAndLabels{
		priority:  priority,
		addLabels: addLabels,
	}
}

// prioritizedLabels is a list of labels sorted by priority. It implements sort.Interface
// to assist with this.
type prioritizedLabels []priorityAndLabels

func (p prioritizedLabels) Len() int           { return len(p) }
func (p prioritizedLabels) Less(i, j int) bool { return p[i].priority < p[j].priority }
func (p prioritizedLabels) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
