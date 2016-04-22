package events

import (
	"github.com/gravitational/teleport/lib/utils"
	"gopkg.in/check.v1"
	"testing"
)

const (
	userBob = "bob"
	userTom = "tom"

	addrBob = "10.0.10.12"
	addrTom = "10.0.10.10"

	addrServerLuna = "10.1.0.80"
	addrServerMars = "10.1.0.81"
)

var (
	cmdLs = []string{"/bin/ls", "-l", "/etc"}
)

type AuditTestSuite struct{}

// bootstrap check
func TestAuditLog(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&AuditTestSuite{})

func (a *AuditTestSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
}

func (a *AuditTestSuite) TestEverything(c *check.C) {
	al, _ := NewAuditLog()
	// open session logger, execute command and close
	sl := al.NewSessionLogger("s1", userBob, addrBob, addrServerLuna)
	sl.OnExec(cmdLs)
	sl.Close()
}
