// +build !cgo

package shell

const (
	DefaultShell = "/bin/sh"
)

func GetLoginShell(username string) (string, error) {
  return DefaultShell, nil
}
