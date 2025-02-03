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

package accessmonitoring

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/access/common"
	"github.com/gravitational/teleport/integrations/access/common/teleport"
	"github.com/gravitational/teleport/integrations/lib/logger"
	"github.com/gravitational/teleport/integrations/lib/stringset"
)

const (
	// defaultAccessMonitoringRulePageSize is the default number of rules to retrieve per request
	defaultAccessMonitoringRulePageSize = 1000
)

// RuleHandler stores a cache of Access Monitoring Rules for use with Access Request routing in plugins.
// Must be initialized by calling InitAccessMonitoringRulesCache, a watcher on Acccess Monitoring Rules must pass in new rules using HandleAccessMonitoringRule.
type RuleHandler struct {
	accessMonitoringRules RuleMap

	apiClient  teleport.Client
	pluginType string
	pluginName string

	fetchRecipientCallback func(ctx context.Context, recipient string) (*common.Recipient, error)
	onCacheUpdateCallback  func(Operation types.OpType, name string, rule *accessmonitoringrulesv1.AccessMonitoringRule) error
}

// RuleMap is a concurrent map for access monitoring rules.
type RuleMap struct {
	sync.RWMutex
	// rules are the access monitoring rules being stored.
	rules map[string]*accessmonitoringrulesv1.AccessMonitoringRule
}

// RuleHandlerConfig stores the configuration for RuleHandler
type RuleHandlerConfig struct {
	Client     teleport.Client
	PluginType string
	PluginName string

	// FetchRecipientCallback is a callback that maps recipient strings to plugin Recipients.
	FetchRecipientCallback func(ctx context.Context, recipient string) (*common.Recipient, error)
	// OnCacheUpdateCallback is a callback that is called when a rule in the cache is created or updated.
	OnCacheUpdateCallback func(Operation types.OpType, name string, rule *accessmonitoringrulesv1.AccessMonitoringRule) error
}

// NewRuleHandler returns a new RuleHandler.
func NewRuleHandler(conf RuleHandlerConfig) *RuleHandler {
	return &RuleHandler{
		accessMonitoringRules: RuleMap{
			rules: make(map[string]*accessmonitoringrulesv1.AccessMonitoringRule),
		},
		apiClient:              conf.Client,
		pluginType:             conf.PluginType,
		pluginName:             conf.PluginName,
		fetchRecipientCallback: conf.FetchRecipientCallback,
		onCacheUpdateCallback:  conf.OnCacheUpdateCallback,
	}
}

// InitAccessMonitoringRulesCache initializes the cache of Access Monitoring Rules.
func (amrh *RuleHandler) InitAccessMonitoringRulesCache(ctx context.Context) error {
	accessMonitoringRules, err := amrh.getAllAccessMonitoringRules(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	amrh.accessMonitoringRules.Lock()
	defer amrh.accessMonitoringRules.Unlock()
	for _, amr := range accessMonitoringRules {
		if !amrh.ruleApplies(amr) {
			continue
		}
		amrh.accessMonitoringRules.rules[amr.GetMetadata().Name] = amr
		if amrh.onCacheUpdateCallback != nil {
			amrh.onCacheUpdateCallback(types.OpPut, amr.GetMetadata().Name, amr)
		}
	}
	return nil
}

// HandleAccessMonitoringRule checks if a new rule should be stored in the cache and updates accordingly.
// Also removes deleted rules from the cache.
func (amrh *RuleHandler) HandleAccessMonitoringRule(ctx context.Context, event types.Event) error {
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
		if !amrh.ruleApplies(req) {
			delete(amrh.accessMonitoringRules.rules, event.Resource.GetName())
			return nil
		}

		// The notification.name is deprecated. Use plugin.spec.name condition instead.
		if req.GetSpec().GetNotification().GetName() != "" {
			req.Spec.Condition = fmt.Sprintf("plugin.spec.name == %q && %s",
				req.GetSpec().GetNotification().GetName(),
				req.GetSpec().GetCondition())
		}

		amrh.accessMonitoringRules.rules[req.Metadata.Name] = req
		if amrh.onCacheUpdateCallback != nil {
			amrh.onCacheUpdateCallback(types.OpPut, req.GetMetadata().Name, req)
		}
		return nil
	case types.OpDelete:
		delete(amrh.accessMonitoringRules.rules, event.Resource.GetName())
		return nil
	default:
		return trace.BadParameter("unexpected event operation %s", op)
	}
}

// RecipientsFromAccessMonitoringRules returns the recipients that result from the Access Monitoring Rules being applied to the given Access Request.
func (amrh *RuleHandler) RecipientsFromAccessMonitoringRules(ctx context.Context, req types.AccessRequest) *common.RecipientSet {
	log := logger.Get(ctx)
	recipientSet := common.NewRecipientSet()

	for _, rule := range amrh.getAccessMonitoringRules() {
		match, err := MatchAccessRequest(rule.Spec.Condition, req, PluginExpressionEnv{
			Name: amrh.pluginName,
		})
		if err != nil {
			log.WarnContext(ctx, "Failed to parse access monitoring notification rule",
				"error", err,
				"rule", rule.Metadata.Name,
			)
		}
		if !match {
			continue
		}
		for _, recipient := range rule.Spec.Notification.Recipients {
			rec, err := amrh.fetchRecipientCallback(ctx, recipient)
			if err != nil {
				log.WarnContext(ctx, "Failed to fetch plugin recipients based on Access monitoring rule recipients", "error", err)
				continue
			}
			recipientSet.Add(*rec)
		}
	}
	return &recipientSet
}

// RawRecipientsFromAccessMonitoringRules returns the recipients that result from the Access Monitoring Rules being applied to the given Access Request without converting to the rich recipient type.
func (amrh *RuleHandler) RawRecipientsFromAccessMonitoringRules(ctx context.Context, req types.AccessRequest) []string {
	log := logger.Get(ctx)
	recipientSet := stringset.New()
	for _, rule := range amrh.getAccessMonitoringRules() {
		match, err := MatchAccessRequest(rule.Spec.Condition, req, PluginExpressionEnv{
			Name: amrh.pluginName,
		})
		if err != nil {
			log.WarnContext(ctx, "Failed to parse access monitoring notification rule",
				"error", err,
				"rule", rule.Metadata.Name,
			)
		}
		if !match {
			continue
		}
		for _, recipient := range rule.Spec.Notification.Recipients {
			recipientSet.Add(recipient)
		}
	}
	return recipientSet.ToSlice()
}

func (amrh *RuleHandler) getAllAccessMonitoringRules(ctx context.Context) ([]*accessmonitoringrulesv1.AccessMonitoringRule, error) {
	var resources []*accessmonitoringrulesv1.AccessMonitoringRule
	var nextToken string
	for {
		var page []*accessmonitoringrulesv1.AccessMonitoringRule
		var err error
		page, nextToken, err = amrh.apiClient.ListAccessMonitoringRulesWithFilter(ctx, defaultAccessMonitoringRulePageSize, nextToken, []string{types.KindAccessRequest}, amrh.pluginName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, amr := range page {
			if !amrh.ruleApplies(amr) {
				continue
			}
			resources = append(resources, amr)
		}

		if nextToken == "" {
			break
		}
	}
	return resources, nil
}

func (amrh *RuleHandler) getAccessMonitoringRules() map[string]*accessmonitoringrulesv1.AccessMonitoringRule {
	amrh.accessMonitoringRules.RLock()
	defer amrh.accessMonitoringRules.RUnlock()
	return maps.Clone(amrh.accessMonitoringRules.rules)
}

func (amrh *RuleHandler) ruleApplies(amr *accessmonitoringrulesv1.AccessMonitoringRule) bool {
	return slices.ContainsFunc(amr.Spec.Subjects, func(subject string) bool {
		return subject == types.KindAccessRequest
	})
}
