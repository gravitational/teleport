package constants

import (
	"time"
)

const (
	// ProPlan is the "pro" product name
	ProPlan = "Teleport Pro"
	// BusinessPlan is the "business" product name
	BusinessPlan = "Teleport Business"
	// EnterprisePlan is the "enterprise" product name
	EnterprisePlan = "Teleport Enterprise"
	// EnterpriseAWSPlan is the "enterprise" product name for AWS Marketplace
	EnterpriseAWSPlan = "Teleport Enterprise AWS"

	// LicenseCheckInterval is the time interval the license is checked when generating license warning alerts.
	LicenseCheckInterval = time.Hour

	// LicenseWarningInterval is the duration before a license expiry in which license alerts are created.
	LicenseWarningInterval = time.Hour * 24 * 90

	// LicenseGraceInterval is the duration after license expiry that we allow licensed features to
	// keep working. Features can be disabled if the license expire time plus the LicenseGraceInterval
	// is reached.
	LicenseGraceInterval = time.Hour * 24 * 30

	// LicenseWarningAlertIDFormat is the alert ID format for license warnings.
	LicenseWarningAlertIDFormat = "license-warning"

	// LicenseWarningMessageFormat is the format for alert message generated when a license is near expiring.
	// The `%s` format will be either "today", "in 1 day" or "in N days".
	LicenseWarningMessageFormat = "Your Teleport Enterprise Edition license will expire %s on one or more of your auth servers. Please reach out to mailto:licenses@goteleport.com to obtain a new license. Inaction may lead to unplanned outage or degraded performance and support."

	// LicenseExpiredMessageFormat is the format for the alert message generated when a license has expired.
	// The `%s` format will be either "today", "in 1 day" or "in N days".
	LicenseExpiredMessageFormat = "Your Teleport Enterprise Edition license has expired on one or more of your auth servers. Enterprise features including SSO will stop working %s. Please reach out to licenses@goteleport.com to update the license. Inaction will lead to enterprise features being disabled and limited support."

	// LicenseDisabledMessageFormat is the format for the alert message generated when a license has been disabled.
	LicenseDisabledMessageFormat = "Your Teleport Enterprise Edition license has expired on one or more of your auth servers. Enterprise features including SSO have been disabled. Please reach out to licenses@goteleport.com to update the license and restore enterprise features."
)
