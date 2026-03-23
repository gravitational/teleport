package utils

import (
	"github.com/gravitational/teleport/session/common/netutils"
)

//go:fix inline
func IsOKNetworkError(err error) bool { return netutils.IsOKNetworkError(err) }
