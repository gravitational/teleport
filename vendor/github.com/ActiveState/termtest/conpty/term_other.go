// +build !windows

package conpty

func InitTerminal(_ bool) (func(), error) {
	return func() {}, nil
}
