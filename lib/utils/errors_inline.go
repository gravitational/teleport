package utils

import (
	"github.com/gravitational/teleport/session/common/netutils"
)

//go:fix inline
func IsUseOfClosedNetworkError(err error) bool { return netutils.IsUseOfClosedNetworkError(err) }

//go:fix inline
func IsFailedToSendCloseNotifyError(err error) bool {
	return netutils.IsFailedToSendCloseNotifyError(err)
}

//go:fix inline
func IsOKNetworkError(err error) bool { return netutils.IsOKNetworkError(err) }

//go:fix inline
func IsConnectionRefused(err error) bool { return netutils.IsConnectionRefused(err) }

//go:fix inline
func IsUntrustedCertErr(err error) bool { return netutils.IsUntrustedCertErr(err) }

//go:fix inline
func CanExplainNetworkError(err error) (string, bool) { return netutils.CanExplainNetworkError(err) }

//go:fix inline
const SelfSignedCertsMsg = netutils.SelfSignedCertsMsg
