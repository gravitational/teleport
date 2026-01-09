//go:build embed_sshd_helper && linux && 386

package teleport

import _ "embed"

//go:embed build/linux_386/teleport-sshd-helper
var SSHDHelperBinary string
