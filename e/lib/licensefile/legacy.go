package licensefile

import (
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/constants"
)

// LegacyLicense represents legacy license information
type LegacyLicense struct {
	// Metadata is an arbitrary customer metadata
	Metadata string `json:"metadata,omitempty"`
	// AccountID is the ID of the account the license was issued for
	AccountID string `json:"account_id,omitempty"`
	// Expiration is expiration time for the license
	Expiration time.Time `json:"expiration"`
	// ProductName is the name of the product the license is for
	ProductName string `json:"product_name,omitempty"`
}

// ToV3 converts LegacyLicense to V3 version
func (l *LegacyLicense) ToV3() (types.License, error) {
	// if it's a legacy license, implement migration to the new version
	licenseV3, err := types.NewLicense(l.ProductName, types.LicenseSpecV3{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	licenseV3.SetExpiry(l.Expiration)
	switch l.ProductName {
	// Pro and Business plans are the same in the way
	// that they only turn on tracking
	case constants.ProPlan, constants.BusinessPlan:
		licenseV3.SetReportsUsage(types.NewBool(true))
		return licenseV3, nil
	case constants.EnterprisePlan:
		// old enterprise plan allows everything except kubernetes
		return licenseV3, nil
	case constants.EnterpriseAWSPlan:
		licenseV3.SetAWSAccountID(l.AccountID)
		licenseV3.SetAWSProductID(l.Metadata)
		return licenseV3, nil
	case "":
		// plan is empty, assume that it's a new style License resource
	default:
		// unrecognized plan, return error
		return nil, trace.BadParameter("unrecognized legacy product name %q", l.ProductName)
	}

	return licenseV3, nil

}
