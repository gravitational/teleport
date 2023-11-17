package okta

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

var interpolationRegex = regexp.MustCompile(`\$\d+`)

// regexAndPriorityLabels contains a regex and associated priority labels.
type regexAndPriorityLabels struct {
	regex             *regexp.Regexp
	priorityAndLabels priorityAndLabels
}

// buildImportRuleMappings will build the import rule mappings used to determine
// which labels to apply to groups and applications synchronized by this service.
// This is intended to be run once per synchronization, and the mappings will
// then be used for the entirety of the synchronization run.
func (s *Service) buildImportRuleMappings(ctx context.Context) error {
	s.labelMu.Lock()
	defer s.labelMu.Unlock()

	// Clear out the existing caches
	s.groupIRMapping = map[string]prioritizedLabels{}
	s.applicationIRMapping = map[string]prioritizedLabels{}
	s.groupNameRegexes = []regexAndPriorityLabels{}
	s.appNameRegexes = []regexAndPriorityLabels{}

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
					if ok, regexes := match.GetGroupNameRegexes(); ok {
						for _, regex := range regexes {
							compiledRegex, err := utils.CompileExpression(regex)
							if err != nil {
								return trace.Wrap(err)
							}
							s.groupNameRegexes = append(s.groupNameRegexes, regexAndPriorityLabels{
								regex:             compiledRegex,
								priorityAndLabels: p,
							})
						}
					}
					if ok, regexes := match.GetAppNameRegexes(); ok {
						for _, regex := range regexes {
							compiledRegex, err := utils.CompileExpression(regex)
							if err != nil {
								return trace.Wrap(err)
							}
							s.appNameRegexes = append(s.appNameRegexes, regexAndPriorityLabels{
								regex:             compiledRegex,
								priorityAndLabels: p,
							})
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
func (s *Service) getGroupLabels(groupID, groupName string) (map[string]string, error) {
	s.labelMu.RLock()
	existingLabels := s.groupIRMapping[groupID]
	groupNameRegexes := s.groupNameRegexes
	s.labelMu.RUnlock()

	groupLabels := make(prioritizedLabels, len(existingLabels))
	copy(groupLabels, existingLabels)

	regexesMatch := false
	for _, regexAndLabel := range groupNameRegexes {
		matches := regexAndLabel.regex.FindStringSubmatch(groupName)
		if len(matches) > 0 {
			regexesMatch = true
			interpolatedLabels, err := interpolatedLabels(matches, regexAndLabel.priorityAndLabels)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			groupLabels = append(groupLabels, interpolatedLabels)
		}
	}

	if regexesMatch {
		sort.Sort(groupLabels)
	}

	return aggregateLabels(groupLabels), nil
}

// getApplicationLabels will return the application labels for the given application ID.
func (s *Service) getApplicationLabels(applicationID, applicationName string) (map[string]string, error) {
	s.labelMu.RLock()
	existingLabels := s.applicationIRMapping[applicationID]
	appNameRegexes := s.appNameRegexes
	s.labelMu.RUnlock()

	appLabels := make(prioritizedLabels, len(existingLabels))
	copy(appLabels, existingLabels)

	regexesMatch := false
	for _, regexAndLabel := range appNameRegexes {
		matches := regexAndLabel.regex.FindStringSubmatch(applicationName)
		if matches != nil {
			regexesMatch = true
			interpolatedLabels, err := interpolatedLabels(matches, regexAndLabel.priorityAndLabels)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			appLabels = append(appLabels, interpolatedLabels)
		}
	}

	if regexesMatch {
		sort.Sort(appLabels)
	}

	return aggregateLabels(appLabels), nil
}

// interpolatedLabels will interpolate matches from regexes into the prioritized labels.
func interpolatedLabels(matches []string, label priorityAndLabels) (priorityAndLabels, error) {
	interpolatedLabels := map[string]string{}

	numMatches := len(matches)
	for k, v := range label.addLabels {
		interpolations := interpolationRegex.FindAllString(v, -1)
		label := v

		for _, interpolation := range interpolations {
			if len(interpolation) < 2 {
				return priorityAndLabels{}, trace.BadParameter("interpolation match needs to be greater than 1 element long")
			}
			matchNum, err := strconv.Atoi(interpolation[1:])
			if err != nil {
				return priorityAndLabels{}, trace.Wrap(err)
			}
			if matchNum >= numMatches {
				return priorityAndLabels{}, trace.BadParameter("no match found for string interpolation %d", matchNum)
			}
			label = strings.ReplaceAll(label, interpolation, matches[matchNum])
		}

		interpolatedLabels[k] = label
	}

	return newPriorityAndLabel(label.priority, interpolatedLabels), nil
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
