//go:build embed_sshd_helper && linux && arm64

package teleport

import _ "embed"

//go:embed build/linux_arm64/teleport-sshd-helper.gz
var SSHDHelperBinaryGZ string
