package services

import (
	"github.com/gravitational/trace"

	webhookv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/webhook/v1"
)

// ValidateWebhook checks resource-specific invariants.
func ValidateWebhook(webhook *webhookv1.Webhook) error {
	if webhook == nil {
		return trace.BadParameter("webhook is nil")
	}
	if webhook.GetMetadata() == nil {
		return trace.BadParameter("metadata is required")
	}
	if webhook.GetMetadata().GetName() == "" {
		return trace.BadParameter("name is required")
	}
	if webhook.GetSpec() == nil {
		return trace.BadParameter("spec is required")
	}
	if webhook.GetSpec().GetUrl() == "" {
		return trace.BadParameter("url is required")
	}
	return nil
}
