package srv

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/utils"
)

type lsSubsys struct {
	path string
}

func parseLSSubsys(name string) (*lsSubsys, error) {
	out := regexp.MustCompile("ls:(.+)").FindStringSubmatch(name)
	if len(out) != 2 {
		return nil, fmt.Errorf("invalid format for ls:", name, len(out))
	}
	return &lsSubsys{
		path: out[1],
	}, nil
}

func (l *lsSubsys) String() string {
	return fmt.Sprintf("lsSubsys(path=%v)", l.path)
}

func (l *lsSubsys) execute(sconn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request, ctx *ctx) error {
	log.Infof("%v %v execute()", ctx, l)

	fis, err := ioutil.ReadDir(l.path)
	if err != nil {
		log.Errorf("%v error: %v", l, err)
		return err
	}

	out := make([]utils.FileNode, len(fis))

	for i, fi := range fis {
		out[i] = utils.FileNode{
			Parent: l.path,
			Name:   fi.Name(),
			Size:   fi.Size(),
			Mode:   int64(fi.Mode()),
			Dir:    fi.IsDir(),
		}
	}

	bytes, err := json.Marshal(out)
	if err != nil {
		log.Errorf("%v error: %v", l, err)
		return err
	}
	_, err = ch.Write(bytes)
	if err != nil {
		log.Errorf("%v error: %v", l, err)
		return err
	}
	return err
}
