package proto

// import (
// 	time "time"

// 	types "github.com/gravitational/teleport/api/types"
// 	"github.com/gravitational/teleport/api/utils"
// )

// func (a *IdentityCenterAccount) SetSubKind(k string) {
// 	a.SubKind = k
// }

// func (a *IdentityCenterAccount) GetName() string {
// 	return a.Metadata.Name
// }

// // SetName sets the name of the resource
// func (a *IdentityCenterAccount) SetName(n string) {
// 	a.Metadata.Name = n
// }

// // Expiry returns object expiry setting
// func (a *IdentityCenterAccount) Expiry() time.Time {
// 	if a.Metadata.Expires == nil {
// 		return time.Time{}
// 	}
// 	return *a.Metadata.Expires
// }

// // SetExpiry sets object expiry
// func (a *IdentityCenterAccount) SetExpiry(expiry time.Time) {
// 	a.Metadata.Expires = &expiry
// }

// // GetRevision returns the revision
// func (a *IdentityCenterAccount) GetRevision() string {
// 	panic("IdentityCenterAccount does not implement GetRevision()")
// }

// // SetRevision sets the revision
// func (a *IdentityCenterAccount) SetRevision(string) {
// 	panic("IdentityCenterAccount does not implement SetRevision()")
// }

// func (a *IdentityCenterAccount) Origin() string {
// 	return a.Metadata.Labels[types.OriginLabel]
// }

// func (a *IdentityCenterAccount) SetOrigin(origin string) {
// 	a.Metadata.Labels[types.OriginLabel] = origin
// }

// func (a *IdentityCenterAccount) GetLabel(key string) (value string, ok bool) {
// 	v, ok := a.Metadata.Labels[key]
// 	return v, ok
// }

// // GetAllLabels returns all resource's labels.
// func (a *IdentityCenterAccount) GetAllLabels() map[string]string {
// 	return a.Metadata.Labels
// }

// // GetStaticLabels returns the resource's static labels.
// func (a *IdentityCenterAccount) GetStaticLabels() map[string]string {
// 	return a.Metadata.Labels
// }

// // SetStaticLabels sets the resource's static labels.
// func (a *IdentityCenterAccount) SetStaticLabels(sl map[string]string) {
// 	a.Metadata.Labels = sl
// }

// // MatchSearch goes through select field values of a resource
// // and tries to match against the list of search values.
// func (a *IdentityCenterAccount) MatchSearch(searchValues []string) bool {
// 	fieldVals := append(utils.MapToStrings(a.GetAllLabels()), a.GetName())
// 	return types.MatchSearch(fieldVals, searchValues, nil)
// }
