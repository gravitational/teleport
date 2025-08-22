package servicecfg

import "time"

// IdentityCenterConfig holds configurable parameters for the IdentityCenter integration
type IdentityCenterConfig struct {
	// EventBatchDuration specifies how long to to collect events before acting
	// on them. Shorter durations make the service more responsive, but longer
	// durations are able to discard more work and are thus more efficient.
	EventBatchDuration time.Duration

	// AccountAssignmentRecalculationInterval is the interval between full
	// assignment recalculations for all Users and Account Assignments.
	AccountAssignmentRecalculationInterval time.Duration

	// ProvisioningStateRefreshInterval determines the interval between full
	// state refreshes (i.e. checks if the principal needs to be re-provisioned)
	// in the Identity Center SCIM provisioner.
	ProvisioningStateRefreshInterval time.Duration
}
