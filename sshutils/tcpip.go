package sshutils

import (
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
)

type DirectTCPIPReq struct {
	Host string
	Port uint32

	Orig     string
	OrigPort uint32
}

func ParseDirectTCPIPReq(data []byte) (*DirectTCPIPReq, error) {
	var r DirectTCPIPReq
	if err := ssh.Unmarshal(data, &r); err != nil {
		log.Infof("failed to parse Direct TCP IP request: %v", err)
		return nil, err
	}
	return &r, nil
}
