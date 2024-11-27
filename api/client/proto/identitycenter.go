package proto

import (
	time "time"

	types "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
)

func (a *IdentityCenterAccountAssignment) SetSubKind(k string) {
	a.SubKind = k
}

func (a *IdentityCenterAccountAssignment) GetName() string {
	return a.Metadata.Name
}

// SetName sets the name of the resource
func (a *IdentityCenterAccountAssignment) SetName(n string) {
	a.Metadata.Name = n
}

// Expiry returns object expiry setting
func (a *IdentityCenterAccountAssignment) Expiry() time.Time {
	if a.Metadata.Expires == nil {
		return time.Time{}
	}
	return *a.Metadata.Expires
}

// SetExpiry sets object expiry
func (a *IdentityCenterAccountAssignment) SetExpiry(expiry time.Time) {
	a.Metadata.Expires = &expiry
}

// GetRevision returns the revision
func (a *IdentityCenterAccountAssignment) GetRevision() string {
	panic("IdentityCenterAccountAssignment does not implement GetRevision()")
}

// SetRevision sets the revision
func (a *IdentityCenterAccountAssignment) SetRevision(string) {
	panic("IdentityCenterAccountAssignment does not implement SetRevision()")
}

func (a *IdentityCenterAccountAssignment) Origin() string {
	return a.Metadata.Labels[types.OriginLabel]
}

func (a *IdentityCenterAccountAssignment) SetOrigin(origin string) {
	a.Metadata.Labels[types.OriginLabel] = origin
}

func (a *IdentityCenterAccountAssignment) GetLabel(key string) (value string, ok bool) {
	v, ok := a.Metadata.Labels[key]
	return v, ok
}

// GetAllLabels returns all resource's labels.
func (a *IdentityCenterAccountAssignment) GetAllLabels() map[string]string {
	return a.Metadata.Labels
}

// GetStaticLabels returns the resource's static labels.
func (a *IdentityCenterAccountAssignment) GetStaticLabels() map[string]string {
	return a.Metadata.Labels
}

// SetStaticLabels sets the resource's static labels.
func (a *IdentityCenterAccountAssignment) SetStaticLabels(sl map[string]string) {
	a.Metadata.Labels = sl
}

// MatchSearch goes through select field values of a resource
// and tries to match against the list of search values.
func (a *IdentityCenterAccountAssignment) MatchSearch(searchValues []string) bool {
	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName())
	return types.MatchSearch(fieldVals, searchValues, nil)
}
