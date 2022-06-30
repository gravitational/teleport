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

	// controlPlaneAPIPort is the default control plane API port
	controlPlaneAPIPort = "443"

	// controlPlaneAPIHost is the default control plane hostname
	controlPlaneAPIHost = "dashboard-api.gravitational.com"

	// MaxControlPlaneUnreachableHours is the number hours after which failure
	// to contact the control plane violates Terms of Service
	MaxControlPlaneUnreachableHours = 48
	// MaxControlPlaneUnreachableDuration is a duration value of MaxControlPlaneUnreachableHours
	MaxControlPlaneUnreachableDuration = MaxControlPlaneUnreachableHours * time.Hour

	// ReportingInterval is how often Teleport Pro reaches out to the
	// control plane to perform a license check and report usage
	ReportingInterval = 6 * time.Hour

	// HeartbeatInterval is how often Teleport Pro records usage and
	// enforces the validity of the current license
	HeartbeatInterval = 5 * time.Minute

	// TeleportSupportURL is company support website URL
	TeleportSupportURL = "https://support.goteleport.com"
	// TeleportDownloadPortalURL is Teleport Download Portal website URL
	TeleportDownloadPortalURL = "https://dashboard.gravitational.com"
)

var (
	// APIHostEnvVar is used to "flip" the URL of Houston API to
	// point it to staging/development servers
	APIHostEnvVar = "HOUSTON_HOSTPORT"
)
