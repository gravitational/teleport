//go:build boringcrypto

package native

import "crypto/boring"

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
//
// It's possible to enable the boringcrypto GOEXPERIMENT (which will enable the
// boringcrypto build tag) even on platforms that don't support the boringcrypto
// module, which results in crypto packages being available and working, but not
// actually using a certified cryptographic module, so we have to check
// [boring.Enabled] even if this is compiled in.
func IsBoringBinary() bool {
	return boring.Enabled()
}
