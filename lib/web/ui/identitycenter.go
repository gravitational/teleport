package ui

// import (
// 	"github.com/gravitational/teleport/api/client/proto"
// 	"github.com/gravitational/teleport/lib/services"
// )

// type IdentityCenterPermissionSet struct {
// 	Name            string `json:"name"`
// 	ARN             string `json:"arn"`
// 	RequiresRequest bool   `json:"requiresRequest,omitempty"`
// }

// type IdentityCenterAccount struct {
// 	// Kind is the kind of resource. Used to parse which kind in a list of unified resources in the UI
// 	Kind string `json:"kind"`

// 	// ID is the AWS-assigned ID of the account
// 	ID string `json:"id"`

// 	// ARN is the ARN for the account
// 	ARN string `json:"arn"`

// 	// Name is the name of the Account.
// 	Name string `json:"name"`

// 	// Description is the app description.
// 	Description string `json:"description"`

// 	PermissionSets []IdentityCenterPermissionSet `json:"permission_sets"`

// 	Labels []Label `json:"labels"`

// 	RequiresRequest bool `json:"requiresRequest,omitempty"`
// }

// func MakeIdentityCenterAccount(acct *proto.IdentityCenterAccount, accessChecker services.AccessChecker, requiresRequest bool) *IdentityCenterAccount {
// 	pss := make([]IdentityCenterPermissionSet, len(acct.PermissionSets))
// 	for i, src := range acct.PermissionSets {
// 		pss[i] = IdentityCenterPermissionSet{
// 			Name:            src.Name,
// 			ARN:             src.ARN,
// 			RequiresRequest: src.RequiresRequest,
// 		}
// 	}

// 	return &IdentityCenterAccount{
// 		Kind:            acct.Kind,
// 		ID:              acct.ID,
// 		ARN:             acct.ARN,
// 		Name:            acct.AccountName,
// 		Description:     acct.Description,
// 		Labels:          makeLabels(acct.GetAllLabels()),
// 		RequiresRequest: requiresRequest,
// 		PermissionSets:  pss,
// 	}
// }
