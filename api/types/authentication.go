/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// AuthPreference defines the authentication preferences for a specific
// cluster. It defines the type (local, oidc) and second factor (off, otp, oidc).
// AuthPreference is a configuration resource, never create more than one instance
// of it.
type AuthPreference interface {
	// Resource provides common resource properties.
	ResourceWithOrigin

	// GetType gets the type of authentication: local, saml, or oidc.
	GetType() string
	// SetType sets the type of authentication: local, saml, or oidc.
	SetType(string)

	// GetSecondFactor gets the type of second factor.
	GetSecondFactor() constants.SecondFactorType
	// SetSecondFactor sets the type of second factor.
	SetSecondFactor(constants.SecondFactorType)
	// IsSecondFactorEnforced checks if second factor is enforced
	// (not disabled or set to optional).
	IsSecondFactorEnforced() bool
	// IsSecondFactorTOTPAllowed checks if users are allowed to register TOTP devices.
	IsSecondFactorTOTPAllowed() bool
	// IsSecondFactorU2FAllowed checks if users are allowed to register U2F devices.
	IsSecondFactorU2FAllowed() bool

	// GetConnectorName gets the name of the OIDC or SAML connector to use. If
	// this value is empty, we fall back to the first connector in the backend.
	GetConnectorName() string
	// SetConnectorName sets the name of the OIDC or SAML connector to use. If
	// this value is empty, we fall back to the first connector in the backend.
	SetConnectorName(string)

	// GetU2F gets the U2F configuration settings.
	GetU2F() (*U2F, error)
	// SetU2F sets the U2F configuration settings.
	SetU2F(*U2F)

	// GetWebauthn returns the Webauthn configuration settings.
	GetWebauthn() (*Webauthn, error)
	// SetWebauthn sets the Webauthn configuration settings.
	SetWebauthn(*Webauthn)

	// GetRequireSessionMFA returns true when all sessions in this cluster
	// require an MFA check.
	GetRequireSessionMFA() bool

	// GetDisconnectExpiredCert returns disconnect expired certificate setting
	GetDisconnectExpiredCert() bool
	// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
	SetDisconnectExpiredCert(bool)

	// GetAllowLocalAuth gets if local authentication is allowed.
	GetAllowLocalAuth() bool
	// SetAllowLocalAuth sets if local authentication is allowed.
	SetAllowLocalAuth(bool)

	// GetMessageOfTheDay fetches the MOTD
	GetMessageOfTheDay() string
	// SetMessageOfTheDay sets the MOTD
	SetMessageOfTheDay(string)

	// GetLockingMode gets the cluster-wide locking mode default.
	GetLockingMode() constants.LockingMode
	// SetLockingMode sets the cluster-wide locking mode default.
	SetLockingMode(constants.LockingMode)

	// String represents a human readable version of authentication settings.
	String() string
}

// NewAuthPreference is a convenience method to to create AuthPreferenceV2.
func NewAuthPreference(spec AuthPreferenceSpecV2) (AuthPreference, error) {
	return newAuthPreferenceWithLabels(spec, map[string]string{})
}

// NewAuthPreferenceFromConfigFile is a convenience method to create
// AuthPreferenceV2 labelled as originating from config file.
func NewAuthPreferenceFromConfigFile(spec AuthPreferenceSpecV2) (AuthPreference, error) {
	return newAuthPreferenceWithLabels(spec, map[string]string{
		OriginLabel: OriginConfigFile,
	})
}

// NewAuthPreferenceWithLabels is a convenience method to create
// AuthPreferenceV2 with a specific map of labels.
func newAuthPreferenceWithLabels(spec AuthPreferenceSpecV2, labels map[string]string) (AuthPreference, error) {
	pref := &AuthPreferenceV2{
		Metadata: Metadata{
			Labels: labels,
		},
		Spec: spec,
	}
	if err := pref.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return pref, nil
}

// DefaultAuthPreference returns the default authentication preferences.
func DefaultAuthPreference() AuthPreference {
	authPref, _ := newAuthPreferenceWithLabels(AuthPreferenceSpecV2{}, map[string]string{
		OriginLabel: OriginDefaults,
	})
	return authPref
}

// GetVersion returns resource version.
func (c *AuthPreferenceV2) GetVersion() string {
	return c.Version
}

// GetName returns the name of the resource.
func (c *AuthPreferenceV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the name of the resource.
func (c *AuthPreferenceV2) SetName(e string) {
	c.Metadata.Name = e
}

// SetExpiry sets expiry time for the object.
func (c *AuthPreferenceV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expiry returns object expiry setting.
func (c *AuthPreferenceV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// GetMetadata returns object metadata.
func (c *AuthPreferenceV2) GetMetadata() Metadata {
	return c.Metadata
}

// GetResourceID returns resource ID.
func (c *AuthPreferenceV2) GetResourceID() int64 {
	return c.Metadata.ID
}

// SetResourceID sets resource ID.
func (c *AuthPreferenceV2) SetResourceID(id int64) {
	c.Metadata.ID = id
}

// Origin returns the origin value of the resource.
func (c *AuthPreferenceV2) Origin() string {
	return c.Metadata.Origin()
}

// SetOrigin sets the origin value of the resource.
func (c *AuthPreferenceV2) SetOrigin(origin string) {
	c.Metadata.SetOrigin(origin)
}

// GetKind returns resource kind.
func (c *AuthPreferenceV2) GetKind() string {
	return c.Kind
}

// GetSubKind returns resource subkind.
func (c *AuthPreferenceV2) GetSubKind() string {
	return c.SubKind
}

// SetSubKind sets resource subkind.
func (c *AuthPreferenceV2) SetSubKind(sk string) {
	c.SubKind = sk
}

// GetType returns the type of authentication.
func (c *AuthPreferenceV2) GetType() string {
	return c.Spec.Type
}

// SetType sets the type of authentication.
func (c *AuthPreferenceV2) SetType(s string) {
	c.Spec.Type = s
}

// GetSecondFactor returns the type of second factor.
func (c *AuthPreferenceV2) GetSecondFactor() constants.SecondFactorType {
	return c.Spec.SecondFactor
}

// SetSecondFactor sets the type of second factor.
func (c *AuthPreferenceV2) SetSecondFactor(s constants.SecondFactorType) {
	c.Spec.SecondFactor = s
}

// IsSecondFactorEnforced checks if second factor is enforced (not disabled or set to optional).
func (c *AuthPreferenceV2) IsSecondFactorEnforced() bool {
	return c.Spec.SecondFactor != constants.SecondFactorOff && c.Spec.SecondFactor != constants.SecondFactorOptional
}

// IsSecondFactorTOTPAllowed checks if users are allowed to register TOTP devices.
func (c *AuthPreferenceV2) IsSecondFactorTOTPAllowed() bool {
	return c.Spec.SecondFactor == constants.SecondFactorOTP || c.Spec.SecondFactor == constants.SecondFactorOptional || c.Spec.SecondFactor == constants.SecondFactorOn
}

// IsSecondFactorU2FAllowed checks if users are allowed to register U2F devices.
func (c *AuthPreferenceV2) IsSecondFactorU2FAllowed() bool {
	return c.Spec.SecondFactor == constants.SecondFactorU2F || c.Spec.SecondFactor == constants.SecondFactorOptional || c.Spec.SecondFactor == constants.SecondFactorOn
}

// GetConnectorName gets the name of the OIDC or SAML connector to use. If
// this value is empty, we fall back to the first connector in the backend.
func (c *AuthPreferenceV2) GetConnectorName() string {
	return c.Spec.ConnectorName
}

// SetConnectorName sets the name of the OIDC or SAML connector to use. If
// this value is empty, we fall back to the first connector in the backend.
func (c *AuthPreferenceV2) SetConnectorName(cn string) {
	c.Spec.ConnectorName = cn
}

// GetU2F gets the U2F configuration settings.
func (c *AuthPreferenceV2) GetU2F() (*U2F, error) {
	if c.Spec.U2F == nil {
		return nil, trace.NotFound("U2F is not configured in this cluster, please contact your administrator and ask them to follow https://goteleport.com/docs/access-controls/guides/u2f/")
	}
	return c.Spec.U2F, nil
}

// SetU2F sets the U2F configuration settings.
func (c *AuthPreferenceV2) SetU2F(u2f *U2F) {
	c.Spec.U2F = u2f
}

func (c *AuthPreferenceV2) GetWebauthn() (*Webauthn, error) {
	if c.Spec.Webauthn == nil {
		// TODO(codingllama): Update with configuration guide once we have it.
		return nil, trace.NotFound("Webauthn is not configured in this cluster, please contact your administrator")
	}
	return c.Spec.Webauthn, nil
}

func (c *AuthPreferenceV2) SetWebauthn(w *Webauthn) {
	c.Spec.Webauthn = w
}

// GetRequireSessionMFA returns true when all sessions in this cluster require
// an MFA check.
func (c *AuthPreferenceV2) GetRequireSessionMFA() bool {
	return c.Spec.RequireSessionMFA
}

// GetDisconnectExpiredCert returns disconnect expired certificate setting
func (c *AuthPreferenceV2) GetDisconnectExpiredCert() bool {
	return c.Spec.DisconnectExpiredCert.Value
}

// SetDisconnectExpiredCert sets disconnect client with expired certificate setting
func (c *AuthPreferenceV2) SetDisconnectExpiredCert(b bool) {
	c.Spec.DisconnectExpiredCert = NewBoolOption(b)
}

// GetAllowLocalAuth gets if local authentication is allowed.
func (c *AuthPreferenceV2) GetAllowLocalAuth() bool {
	return c.Spec.AllowLocalAuth.Value
}

// SetAllowLocalAuth gets if local authentication is allowed.
func (c *AuthPreferenceV2) SetAllowLocalAuth(b bool) {
	c.Spec.AllowLocalAuth = NewBoolOption(b)
}

// GetMessageOfTheDay gets the current Message Of The Day. May be empty.
func (c *AuthPreferenceV2) GetMessageOfTheDay() string {
	return c.Spec.MessageOfTheDay
}

// SetMessageOfTheDay sets the current Message Of The Day. May be empty.
func (c *AuthPreferenceV2) SetMessageOfTheDay(motd string) {
	c.Spec.MessageOfTheDay = motd
}

// GetLockingMode gets the cluster-wide locking mode default.
func (c *AuthPreferenceV2) GetLockingMode() constants.LockingMode {
	return c.Spec.LockingMode
}

// SetLockingMode sets the cluster-wide locking mode default.
func (c *AuthPreferenceV2) SetLockingMode(mode constants.LockingMode) {
	c.Spec.LockingMode = mode
}

// setStaticFields sets static resource header and metadata fields.
func (c *AuthPreferenceV2) setStaticFields() {
	c.Kind = KindClusterAuthPreference
	c.Version = V2
	c.Metadata.Name = MetaNameClusterAuthPreference
}

// CheckAndSetDefaults verifies the constraints for AuthPreference.
func (c *AuthPreferenceV2) CheckAndSetDefaults() error {
	c.setStaticFields()
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if c.Spec.Type == "" {
		c.Spec.Type = constants.Local
	}
	if c.Spec.SecondFactor == "" {
		c.Spec.SecondFactor = constants.SecondFactorOTP
	}
	if c.Spec.AllowLocalAuth == nil {
		c.Spec.AllowLocalAuth = NewBoolOption(true)
	}
	if c.Spec.DisconnectExpiredCert == nil {
		c.Spec.DisconnectExpiredCert = NewBoolOption(false)
	}
	if c.Spec.LockingMode == "" {
		c.Spec.LockingMode = constants.LockingModeBestEffort
	}
	if c.Origin() == "" {
		c.SetOrigin(OriginDynamic)
	}

	// make sure type makes sense
	switch c.Spec.Type {
	case constants.Local, constants.OIDC, constants.SAML, constants.Github:
	default:
		return trace.BadParameter("authentication type %q not supported", c.Spec.Type)
	}

	// make sure second factor makes sense
	switch sf := c.Spec.SecondFactor; sf {
	case constants.SecondFactorOff, constants.SecondFactorOTP:
	case constants.SecondFactorU2F:
		if c.Spec.U2F == nil {
			return trace.BadParameter("missing required U2F configuration for second factor type %q", c.Spec.SecondFactor)
		}
		if err := c.Spec.U2F.Check(); err != nil {
			return trace.Wrap(err)
		}
	case constants.SecondFactorWebauthn:
		if c.Spec.Webauthn == nil {
			c.Spec.Webauthn = &Webauthn{} // Try to get the defaults from U2F.
		}
		// Validate U2F, as we take some defaults from it.
		if c.Spec.U2F != nil {
			if err := c.Spec.U2F.Check(); err != nil {
				return trace.Wrap(err)
			}
		}
		if err := c.Spec.Webauthn.CheckAndSetDefaults(sf, c.Spec.U2F); err != nil {
			return trace.Wrap(err)
		}
	case constants.SecondFactorOn, constants.SecondFactorOptional:
		if c.Spec.U2F == nil {
			return trace.BadParameter("missing required U2F configuration for second factor type %q", c.Spec.SecondFactor)
		}
		if err := c.Spec.U2F.Check(); err != nil {
			return trace.Wrap(err)
		}
		if c.Spec.Webauthn == nil {
			c.Spec.Webauthn = &Webauthn{} // Try to get the defaults from U2F.
		}
		if err := c.Spec.Webauthn.CheckAndSetDefaults(sf, c.Spec.U2F); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("second factor type %q not supported", c.Spec.SecondFactor)
	}

	switch c.Spec.LockingMode {
	case constants.LockingModeBestEffort, constants.LockingModeStrict:
	default:
		return trace.BadParameter("locking mode %q not supported", c.Spec.LockingMode)
	}

	return nil
}

// String represents a human readable version of authentication settings.
func (c *AuthPreferenceV2) String() string {
	return fmt.Sprintf("AuthPreference(Type=%q,SecondFactor=%q)", c.Spec.Type, c.Spec.SecondFactor)
}

func (u *U2F) Check() error {
	if u.AppID == "" {
		return trace.BadParameter("u2f configuration missing app_id")
	}
	if len(u.Facets) == 0 {
		return trace.BadParameter("u2f configuration missing facets")
	}
	for _, ca := range u.DeviceAttestationCAs {
		if err := isValidAttestationCert(ca); err != nil {
			return trace.BadParameter("u2f configuration has an invalid attestation CA: %v", err)
		}
	}
	return nil
}

func (w *Webauthn) CheckAndSetDefaults(secondFactor constants.SecondFactorType, u *U2F) error {
	// RPID.
	switch {
	case w.RPID != "": // Explicit RPID
		_, err := url.Parse(w.RPID)
		if err != nil {
			return trace.BadParameter("webauthn rp_id is not a valid URI: %v", err)
		}
	case u != nil && w.RPID == "": // Infer RPID from U2F app_id
		parsedAppID, err := url.Parse(u.AppID)
		if err != nil {
			return trace.BadParameter("webauthn missing rp_id and U2F app_id is not an URL (%v)", err)
		}

		var rpID string
		switch {
		case parsedAppID.Host != "":
			rpID = parsedAppID.Host
			rpID = strings.Split(rpID, ":")[0] // Remove :port, if present
		case parsedAppID.Path == u.AppID:
			// App ID is not a proper URL, take it literally.
			rpID = u.AppID
		default:
			return trace.BadParameter("failed to infer webauthn RPID from U2F App ID (%q)", u.AppID)
		}
		log.Infof("WebAuthn: RPID inferred from U2F configuration: %q", rpID)
		w.RPID = rpID
	default:
		return trace.BadParameter("webauthn configuration missing rp_id")
	}

	// AttestationAllowedCAs.
	switch {
	case u != nil && len(u.DeviceAttestationCAs) > 0 && len(w.AttestationAllowedCAs) == 0 && len(w.AttestationDeniedCAs) == 0:
		log.Infof("WebAuthn: using U2F device attestion CAs as allowed CAs")
		w.AttestationAllowedCAs = u.DeviceAttestationCAs
	default:
		for _, pem := range w.AttestationAllowedCAs {
			if err := isValidAttestationCert(pem); err != nil {
				return trace.BadParameter("webauthn allowed CAs entry invalid: %v", err)
			}
		}
	}

	// AttestationDeniedCAs.
	for _, pem := range w.AttestationDeniedCAs {
		if err := isValidAttestationCert(pem); err != nil {
			return trace.BadParameter("webauthn denied CAs entry invalid: %v", err)
		}
	}

	// Global disable flag.
	if secondFactor == constants.SecondFactorWebauthn && w.Disabled {
		return trace.BadParameter("webauthn cannot be disabled when second_factor = %v", constants.SecondFactorWebauthn)
	}

	return nil
}

func isValidAttestationCert(certOrPath string) error {
	_, err := tlsutils.ParseCertificatePEM([]byte(certOrPath))
	return err
}

// Check validates WebauthnLocalAuth, returning an error if it's not valid.
func (wal *WebauthnLocalAuth) Check() error {
	if len(wal.UserID) == 0 {
		return trace.BadParameter("missing UserID field")
	}
	return nil
}

// NewMFADevice creates a new MFADevice with the given name. Caller must set
// the Device field in the returned MFADevice.
func NewMFADevice(name, id string, addedAt time.Time) *MFADevice {
	return &MFADevice{
		Metadata: Metadata{
			Name: name,
		},
		Id:       id,
		AddedAt:  addedAt,
		LastUsed: addedAt,
	}
}

// setStaticFields sets static resource header and metadata fields.
func (d *MFADevice) setStaticFields() {
	d.Kind = KindMFADevice
	d.Version = V1
}

// CheckAndSetDefaults validates MFADevice fields and populates empty fields
// with default values.
func (d *MFADevice) CheckAndSetDefaults() error {
	d.setStaticFields()
	if err := d.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if d.Id == "" {
		return trace.BadParameter("MFADevice missing ID field")
	}
	if d.AddedAt.IsZero() {
		return trace.BadParameter("MFADevice missing AddedAt field")
	}
	if d.LastUsed.IsZero() {
		return trace.BadParameter("MFADevice missing LastUsed field")
	}
	if d.LastUsed.Before(d.AddedAt) {
		return trace.BadParameter("MFADevice LastUsed field must be earlier than AddedAt")
	}
	if d.Device == nil {
		return trace.BadParameter("MFADevice missing Device field")
	}
	return nil
}

func (d *MFADevice) GetKind() string         { return d.Kind }
func (d *MFADevice) GetSubKind() string      { return d.SubKind }
func (d *MFADevice) SetSubKind(sk string)    { d.SubKind = sk }
func (d *MFADevice) GetVersion() string      { return d.Version }
func (d *MFADevice) GetMetadata() Metadata   { return d.Metadata }
func (d *MFADevice) GetName() string         { return d.Metadata.GetName() }
func (d *MFADevice) SetName(n string)        { d.Metadata.SetName(n) }
func (d *MFADevice) GetResourceID() int64    { return d.Metadata.ID }
func (d *MFADevice) SetResourceID(id int64)  { d.Metadata.SetID(id) }
func (d *MFADevice) Expiry() time.Time       { return d.Metadata.Expiry() }
func (d *MFADevice) SetExpiry(exp time.Time) { d.Metadata.SetExpiry(exp) }

// MFAType returns the human-readable name of the MFA protocol of this device.
func (d *MFADevice) MFAType() string {
	switch d.Device.(type) {
	case *MFADevice_Totp:
		return "TOTP"
	case *MFADevice_U2F:
		return "U2F"
	case *MFADevice_Webauthn:
		return "WebAuthn"
	default:
		return "unknown"
	}
}

func (d *MFADevice) MarshalJSON() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := (&jsonpb.Marshaler{}).Marshal(buf, d)
	return buf.Bytes(), trace.Wrap(err)
}

func (d *MFADevice) UnmarshalJSON(buf []byte) error {
	return jsonpb.Unmarshal(bytes.NewReader(buf), d)
}
