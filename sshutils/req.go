package sshutils

type EnvReqParams struct {
	Name  string
	Value string
}

type WinChangeReqParams struct {
	W     uint32
	H     uint32
	Wpx   uint32
	Hpx   uint32
	Modes string
}

type PTYReqParams struct {
	Env   string
	W     uint32
	H     uint32
	Wpx   uint32
	Hpx   uint32
	Modes string
}

const (
	SessionEnvVar   = "TELEPORT_SESSION"
	SetEnvReq       = "env"
	WindowChangeReq = "window-change"
	PTYReq          = "pty-req"
)
