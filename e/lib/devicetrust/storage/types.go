package storage

import "time"

// storedDeviceCredential represents a devicepb.DeviceCredential in storage.
type storedDeviceCredential struct {
	ID                    string `json:"id"`                      // Required.
	PublicKeyDER          []byte `json:"public_key_der"`          // Required on macos.
	DeviceAttestationType int    `json:"device_attestation_type"` // Required. Same as devicepb.DeviceAttestationType.
	TPMEKCertSerial       string `json:"tpm_ekcert_serial"`       // Optional.
	TPMAKPublic           []byte `json:"tpm_ak_public"`           // Required on windows/linux.
}

// storedDevice (partially) represents a devicepb.Device in storage.
// Written under "device/id/<ID>" keys.
type storedDevice struct {
	OSType       int                     `json:"os_type"`                 // Required. Same as devicepb.OSType.
	AssetTag     string                  `json:"asset_tag"`               // Required.
	CreateTime   time.Time               `json:"create_time"`             // Required.
	UpdateTime   time.Time               `json:"update_time"`             // Required.
	EnrollStatus int                     `json:"enroll_status,omitempty"` // Same as devicepb.EnrollStatus.
	Credential   *storedDeviceCredential `json:"credential,omitempty"`    // Optional. Present if enrolled.
	Source       *storedDeviceSource     `json:"source,omitempty"`        // Optional.
	Profile      *storedDeviceProfile    `json:"profile,omitempty"`       // Optional.
	Owner        string                  `json:"owner,omitempty"`         // Optional (added after v13.2)
}

// deviceRef is stored as reference to a device in manually managed indexes.
type deviceRef struct {
	DeviceID string `json:"device_id"` // Required.
	OSType   int    `json:"os_type"`   // Required. Same as devicepb.OSType.
}

// devicesRef is a stored reference to N devices, used by manually managed
// indexes.
type devicesRef struct {
	Devices []*deviceRef `json:"devices,omitempty"`
}

// storedEnrollToken represents a devicepb.DeviceEnrollToken in storage.
type storedEnrollToken struct {
	HashedToken         []byte `json:"hashed_token"`           // Required.
	CreatedByAutoEnroll bool   `json:"created_by_auto_enroll"` // Optional.
}

// collectedDataOrigin represents the origin of the collected data.
type collectedDataOrigin int

const (
	// originEnrollment is used for collected data acquired during enrollment.
	originEnrollment collectedDataOrigin = iota + 1

	// originAuthentication is used for collected data acquired during device
	// authentication.
	originAuthentication
)

// storedCollectedData represents a devicepb.DeviceCollectedData in storage.
type storedCollectedData struct {
	Origin                  collectedDataOrigin     `json:"origin"`                              // Required.
	CollectTime             time.Time               `json:"collect_time"`                        // Required.
	RecordTime              time.Time               `json:"record_time"`                         // Required.
	OSType                  int                     `json:"os_type"`                             // Required. Same as devicepb.OSType.
	SerialNumber            string                  `json:"serial_number"`                       // Required.
	ModelIdentifier         string                  `json:"model_identifier,omitempty"`          // Optional.
	OSVersion               string                  `json:"os_version,omitempty"`                // Optional.
	OSBuild                 string                  `json:"os_build,omitempty"`                  // Optional.
	OSUsername              string                  `json:"os_username,omitempty"`               // Optional.
	JamfBinaryVersion       string                  `json:"jamf_binary_version,omitempty"`       // Optional.
	MacOSEnrollmentProfiles string                  `json:"macos_enrollment_profiles,omitempty"` // Optional.
	ReportedAssetTag        string                  `json:"reported_asset_tag,omitempty"`        // Optional.
	SystemSerialNumber      string                  `json:"system_serial_number,omitempty"`      // Optional.
	BaseBoardSerialNumber   string                  `json:"base_board_serial_number,omitempty"`  // Optional.
	TPMPlatformAttestation  *tpmPlatformAttestation `json:"tpm_platform_attestation,omitempty"`  // Optional.
	OSID                    string                  `json:"os_id,omitempty"`                     // Optional.
}

// storedDeviceSource represents a devicepb.DeviceSource in storage.
type storedDeviceSource struct {
	Name   string `json:"name"`   // Required.
	Origin int    `json:"origin"` // Required. Same as devicepb.DeviceOrigin.
}

// storedDeviceProfile represents a devicepb.DeviceProfile in storage.
type storedDeviceProfile struct {
	UpdateTime          time.Time `json:"update_time"`                     // Required.
	ModelIdentifier     string    `json:"model_identifier,omitempty"`      // Optional.
	OSVersion           string    `json:"os_version,omitempty"`            // Optional.
	OSBuild             string    `json:"os_build,omitempty"`              // Optional.
	OSBuildSupplemental string    `json:"os_build_supplemental,omitempty"` // Optional.
	OSUsernames         []string  `json:"os_usernames,omitempty"`          // Optional.
	JamfBinaryVersion   string    `json:"jamf_binary_version,omitempty"`   // Optional.
	ExternalID          string    `json:"external_id,omitempty"`           // Optional.
	OSID                string    `json:"os_id,omitempty"`                 // Optional.
}

type tpmPlatformAttestation struct {
	Nonce              []byte                 `json:"nonce"`               // Required.
	PlatformParameters *tpmPlatformParameters `json:"platform_parameters"` // Required.
}

type tpmPlatformParameters struct {
	Quotes   []tpmQuote `json:"quotes"`    // Required.
	PCRs     []tpmPCR   `json:"pcrs"`      // Required.
	EventLog []byte     `json:"event_log"` // Required.
}

type tpmQuote struct {
	Quote     []byte `json:"quote"`     // Required.
	Signature []byte `json:"signature"` // Required.
}

type tpmPCR struct {
	Index     int32  `json:"index"`      // Required.
	Digest    []byte `json:"digest"`     // Required.
	DigestAlg uint64 `json:"digest_alg"` // Required.
}

type webAuthenticationAttemptState int

const (
	_                               webAuthenticationAttemptState = iota // Unspecified.
	webAuthenticationAttemptCreated                                      // DeviceWebToken issued
	webAuthenticationAttemptConfirm                                      // DeviceWebToken spent, DeviceConfirmationToken issued
)

// storedWebAuthenticationAttempt tracks the lifecycle of a device web
// authentication attempt, including both DeviceWebToken and
// DeviceConfirmationToken.
//
// See
// https://github.com/gravitational/teleport.e/blob/master/rfd/0009e-device-trust-web-support.md#device-authentication-attempt.
type storedWebAuthenticationAttempt struct {
	State                 webAuthenticationAttemptState `json:"state"`                             // Required.
	HashedWebToken        []byte                        `json:"hashed_web_token,omitempty"`        // bcrypt-encoded, present on Created state.
	HashedConfirmToken    []byte                        `json:"hashed_confirm_token,omitempty"`    // bcrypt-encoded, present on Confirm state.
	WebSessionID          string                        `json:"web_session_id"`                    // Required.
	User                  string                        `json:"user"`                              // Required.
	BrowserUserAgent      string                        `json:"browser_user_agent"`                // Required.
	BrowserIP             string                        `json:"browser_ip"`                        // Required.
	ExpectedDeviceIDs     []string                      `json:"expected_device_ids"`               // Required.
	AuthenticatedDeviceID string                        `json:"authenticated_device_id,omitempty"` // present on Confirm state.
}
