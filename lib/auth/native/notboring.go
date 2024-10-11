//go:build !boringcrypto

package native

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
//
// The boringcrypto GOEXPERIMENT always sets the boringcrypto build tag, so if
// this is compiled in, we're not using BoringCrypto.
func IsBoringBinary() bool {
	return false
}
