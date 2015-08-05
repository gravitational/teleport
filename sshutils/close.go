package sshutils

import (
	"io"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
)

func CloseAll(closers ...io.Closer) error {
	var err error
	for _, cl := range closers {
		if cl == nil {
			continue
		}
		if e := cl.Close(); e != nil {
			log.Infof("%T close failure: %v", cl, e)
			err = e
		}
	}
	return err
}
