package notification

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/accessmonitoring"
)

type RuleHandlerClient interface {
	ListAccessMonitoringRulesWithFilter(ctx context.Context, req *accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest) ([]*accessmonitoringrulesv1.AccessMonitoringRule, string, error)
}

type RuleHandlerConfig struct {
	// HandlerName specifies the handler name.
	HandlerName string

	// NotificationName specifies the notification name.
	NotificationName string
	// ReviewName specifies the review name.
	ReviewName string

	// Client is the auth service client interface.
	Client RuleHandlerClient

	// Cache is the access monitoring rules cache.
	Cache *accessmonitoring.Cache
}

type RuleHandler struct {
	RuleHandlerConfig

	rules *accessmonitoring.Cache
}

// CheckAndSetDefaults checks and sets default configuration.
func (cfg *RuleHandlerConfig) CheckAndSetDefaults() error {
	if cfg.Client == nil {
		return trace.BadParameter("teleport client is required")
	}
	if cfg.Cache == nil {
		cfg.Cache = accessmonitoring.NewCache()
	}

	return nil
}

func NewRuleHandler(cfg RuleHandlerConfig) (*RuleHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &RuleHandler{
		RuleHandlerConfig: cfg,
		rules:             cfg.Cache,
	}, nil
}

// initialize the access monitoring rules cache.
func (handler *RuleHandler) initialize(ctx context.Context) error {
	err := handler.rules.Initialize(ctx, func(ctx context.Context, pageSize int64, pageToken string) (
		[]*accessmonitoringrulesv1.AccessMonitoringRule,
		string,
		error,
	) {
		req := &accessmonitoringrulesv1.ListAccessMonitoringRulesWithFilterRequest{
			PageSize:         pageSize,
			PageToken:        pageToken,
			Subjects:         []string{types.KindAccessRequest},
			NotificationName: handler.HandlerName,
		}
		page, next, err := handler.Client.ListAccessMonitoringRulesWithFilter(ctx, req)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}

		rules := []*accessmonitoringrulesv1.AccessMonitoringRule{}
		for _, rule := range page {
			if handler.ruleApplies(rule) {
				rules = append(rules, rule)
			}
		}
		return rules, next, nil
	})
	return trace.Wrap(err)
}

// HandleAccessMonitoringRule handles access monitoring rule events.
func (handler *RuleHandler) HandleAccessMonitoringRule(ctx context.Context, event types.Event) error {
	switch event.Type {
	case types.OpInit:
		if err := handler.initialize(ctx); err != nil {
			return trace.Wrap(err)
		}
	case types.OpPut:
		e, ok := event.Resource.(types.Resource153UnwrapperT[*accessmonitoringrulesv1.AccessMonitoringRule])
		if !ok {
			return trace.BadParameter("expected resource type, got %T", event.Resource)
		}
		rule := e.UnwrapT()

		// In the event an existing rule no longer applies we must remove it.
		if !handler.ruleApplies(rule) {
			handler.rules.Delete(rule.GetMetadata().GetName())
			return nil
		}
		handler.rules.Put([]*accessmonitoringrulesv1.AccessMonitoringRule{rule})
	case types.OpDelete:
		handler.rules.Delete(event.Resource.GetName())
	default:
		return trace.BadParameter("unexpected event operation %s", event.Type)
	}
	return nil
}

// ruleApplies returns true if the rule applies to this handler.
func (handler *RuleHandler) ruleApplies(rule *accessmonitoringrulesv1.AccessMonitoringRule) bool {
	if rule.GetSpec().GetNotification().GetName() != handler.HandlerName {
		return false
	}
	return slices.Contains(rule.GetSpec().GetSubjects(), types.KindAccessRequest)
}
