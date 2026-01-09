//go:build embed_sshd_helper && linux && amd64

package teleport

import _ "embed"

//go:embed build/linux_amd64/teleport-sshd-helper
var SSHDHelperBinary string
