//go:build embed_sshd_helper && linux && arm

package teleport

import _ "embed"

//go:embed build/linux_arm/teleport-sshd-helper
var SSHDHelperBinary string
