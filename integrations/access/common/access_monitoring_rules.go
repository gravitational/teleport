/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package common

import (
	"context"
	"maps"
	"sync"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib/logger"
)

const (
	// defaultAccessMonitoringRulePageSize is the default number of rules to retrieve per request
	defaultAccessMonitoringRulePageSize = 10
)

type AccessMonitoringRuleHandler struct {
	accessMonitoringRules AmrMap

	apiClient  teleport.Client
	pluginType string

	ruleAppliesCallback        func(amr *accessmonitoringrulesv1.AccessMonitoringRule) bool
	fetchRecipientCallback     func(ctx context.Context, recipient string) (*Recipient, error)
	matchAccessRequestCallback func(expr string, req types.AccessRequest) (bool, error)
}

// AmrMap is a concurrent map for access monitoring rules.
type AmrMap struct {
	sync.RWMutex
	// rules are the access monitoring rules being stored.
	rules map[string]*accessmonitoringrulesv1.AccessMonitoringRule
}

type AccessMonitoringRuleHandlerConfig struct {
	Client     teleport.Client
	PluginType string

	RuleAppliesCallback        func(amr *accessmonitoringrulesv1.AccessMonitoringRule) bool
	FetchRecipientCallback     func(ctx context.Context, recipient string) (*Recipient, error)
	MatchAccessRequestCallback func(expr string, req types.AccessRequest) (bool, error)
}

func NewAccessMonitoringRuleHandler(conf AccessMonitoringRuleHandlerConfig) *AccessMonitoringRuleHandler {
	return &AccessMonitoringRuleHandler{
		accessMonitoringRules: AmrMap{
			rules: make(map[string]*accessmonitoringrulesv1.AccessMonitoringRule),
		},
		apiClient:                  conf.Client,
		pluginType:                 conf.PluginType,
		ruleAppliesCallback:        conf.RuleAppliesCallback,
		fetchRecipientCallback:     conf.FetchRecipientCallback,
		matchAccessRequestCallback: conf.MatchAccessRequestCallback,
	}
}

func (amrh *AccessMonitoringRuleHandler) InitAccessMonitoringRulesCache(ctx context.Context) error {
	accessMonitoringRules, err := amrh.getAllAccessMonitoringRules(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	amrh.accessMonitoringRules.Lock()
	defer amrh.accessMonitoringRules.Unlock()
	for _, amr := range accessMonitoringRules {
		if !amrh.ruleAppliesCallback(amr) {
			continue
		}
		amrh.accessMonitoringRules.rules[amr.GetMetadata().Name] = amr
	}
	return nil
}

func (amrh *AccessMonitoringRuleHandler) HandleAccessMonitoringRule(ctx context.Context, event types.Event) error {
	if kind := event.Resource.GetKind(); kind != types.KindAccessMonitoringRule {
		return trace.BadParameter("expected %s resource kind, got %s", types.KindAccessMonitoringRule, kind)
	}

	amrh.accessMonitoringRules.Lock()
	defer amrh.accessMonitoringRules.Unlock()
	switch op := event.Type; op {
	case types.OpPut:
		e, ok := event.Resource.(types.Resource153Unwrapper)
		if !ok {
			return trace.BadParameter("expected Resource153Unwrapper resource type, got %T", event.Resource)
		}
		req, ok := e.Unwrap().(*accessmonitoringrulesv1.AccessMonitoringRule)
		if !ok {
			return trace.BadParameter("expected AccessMonitoringRule resource type, got %T", event.Resource)
		}

		// In the event an existing rule no longer applies we must remove it.
		if !amrh.ruleAppliesCallback(req) {
			delete(amrh.accessMonitoringRules.rules, event.Resource.GetName())
			return nil
		}
		amrh.accessMonitoringRules.rules[req.Metadata.Name] = req
		return nil
	case types.OpDelete:
		delete(amrh.accessMonitoringRules.rules, event.Resource.GetName())
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

func (amrh *AccessMonitoringRuleHandler) RecipientsFromAccessMonitoringRules(ctx context.Context, req types.AccessRequest) *RecipientSet {
	log := logger.Get(ctx)
	recipientSet := NewRecipientSet()

	for _, rule := range amrh.getAccessMonitoringRules() {
		match, err := amrh.matchAccessRequestCallback(rule.Spec.Condition, req)
		if err != nil {
			log.WithError(err).WithField("rule", rule.Metadata.Name).
				Warn("Failed to parse access monitoring notification rule")
		}
		if !match {
			continue
		}
		for _, recipient := range rule.Spec.Notification.Recipients {
			rec, err := amrh.fetchRecipientCallback(ctx, recipient)
			if err != nil {
				log.WithError(err).Warn("Failed to fetch plugin recipients based on Access moniotring rule recipients")
				continue
			}
			recipientSet.Add(*rec)
		}
	}
	return &recipientSet
}

func (amrh *AccessMonitoringRuleHandler) getAllAccessMonitoringRules(ctx context.Context) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	var resources []*accessmonitoringrulesv1.AccessMonitoringRule
	var nextToken string
	for {
		var page []*accessmonitoringrulesv1.AccessMonitoringRule
		var err error
		page, nextToken, err = amrh.apiClient.ListAccessMonitoringRulesWithFilter(ctx, defaultAccessMonitoringRulePageSize, nextToken, []string{types.KindAccessRequest}, amrh.pluginType)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, page...)

		if nextToken == "" {
			break
		}
	}
	return resources, nil
}

func (amrh *AccessMonitoringRuleHandler) getAccessMonitoringRules() map[string]*accessmonitoringrulesv1.AccessMonitoringRule {
	amrh.accessMonitoringRules.RLock()
	defer amrh.accessMonitoringRules.RUnlock()
	return maps.Clone(amrh.accessMonitoringRules.rules)
}
