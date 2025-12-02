package oktahandler

// Shim is a resource handler that implements both ProviderUser and ProviderGroup interfaces.
type Shim interface {
	ProviderUser
	ProviderGroup
}
