package constants

import (
	"fmt"
	"time"
)

const (
	// ProPlan is the "pro" product name
	ProPlan = "Teleport Pro"
	// BusinessPlan is the "business" product name
	BusinessPlan = "Teleport Business"
	// EnterprisePlan is the "enterprise" product name
	EnterprisePlan = "Teleport Enterprise"

	// ControlPlaneAPIHost is the control plane hostname
	ControlPlaneAPIHost = "dashboard-api.gravitational.com"
	// ControlPlaneAPIPort is the control plane API port
	ControlPlaneAPIPort = 443

	// MaxControlPlaneUnreachableHours is the number hours after which failure
	// to contact the control plane violates Terms of Service
	MaxControlPlaneUnreachableHours = 48
	// MaxControlPlaneUnreachableDuration is a duration value of MaxControlPlaneUnreachableHours
	MaxControlPlaneUnreachableDuration = MaxControlPlaneUnreachableHours * time.Hour

	// LicenseCheckInterval is how often Teleport Pro reaches out to the
	// control plane to perform a license check
	LicenseCheckInterval = 5 * time.Minute
	// EnforcementInterval is how often enforcement checks are turned on
	EnforcementInterval = 5 * time.Minute

	// GravitationalSupportURL is company support website URL
	GravitationalSupportURL = "https://support.gravitational.com"
	// GravitationalDownloadPortalURL is Teleport Download Portal website URL
	GravitationalDownloadPortalURL = "https://dashboard.gravitational.com"
)

var (
	// ControlPlaneAPIAddr is the control plane host:port
	ControlPlaneAPIAddr = fmt.Sprintf("%v:%v", ControlPlaneAPIHost, ControlPlaneAPIPort)
	// ControlPlaneAPIURL is the control plane API URL
	ControlPlaneAPIURL = fmt.Sprintf("https://%v/api", ControlPlaneAPIAddr)
	// ProPlans is a list of plans that enable "pro" mode
	ProPlans = []string{ProPlan, BusinessPlan}
)
