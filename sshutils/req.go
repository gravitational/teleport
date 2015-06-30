package sshutils

type EnvReq struct {
	Name  string
	Value string
}

const (
	SessionEnvVar = "TELEPORT_SESSION"
	SetEnvReq     = "env"
)
