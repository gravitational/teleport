package main

import (
	"golang.org/x/sys/windows/svc"

	"github.com/gravitational/teleport/lib/vnet"
	tshcommon "github.com/gravitational/teleport/tool/tsh/common"
)

func main() {
	if isService, err := svc.IsWindowsService(); err == nil && isService {
		vnet.ServiceMain()
		return
	}
	tshcommon.Main()
}
