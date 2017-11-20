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

	// MaxControlPlaneUnreachableDuration is amount of time after which failure
	// to contact the control plane violates Terms of Service
	MaxControlPlaneUnreachableDuration = 24 * time.Hour

	// EnforcerHeartbeatPeriod is how often enforcer calls home
	EnforcerHeartbeatPeriod = 2 * time.Second
	// EnforcerEnforcePeriod is how often enforcer turns on enforcement checks
	EnforcerEnforcePeriod = 5 * time.Second
)

var (
	// ControlPlaneAPIAddr is the control plane host:port
	ControlPlaneAPIAddr = fmt.Sprintf("%v:%v", ControlPlaneAPIHost, ControlPlaneAPIPort)
	// ControlPlaneAPIURL is the control plane API URL
	ControlPlaneAPIURL = fmt.Sprintf("https://%v/api", ControlPlaneAPIAddr)
	// ProPlans is a list of plans that enable "pro" mode
	ProPlans = []string{ProPlan, BusinessPlan}
)
