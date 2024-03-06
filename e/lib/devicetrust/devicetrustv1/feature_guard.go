package devicetrustv1

import "github.com/gravitational/trace"

// deviceWebAuthnEnabled controls whether the device web authentication feature
// is enabled by gating authentication and the creation of new DeviceWebTokens.
// TODO(codingllama): Remove once the feature is ready for release.
var deviceWebAuthnEnabled = false

// errDeviceWebAuthnDisabled is returned by device web authentication features
// if it is disabled.
var errDeviceWebAuthnDisabled = &trace.NotImplementedError{
	Message: "device web authentication not implemented",
}
