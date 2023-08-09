package utils

import (
	"github.com/gravitational/teleport/lib/utils/utilsaddr"
)

// TODO(jent): Remove after addr changes have been fully adopted
type NetAddr = utilsaddr.NetAddr

var ParseHostPortAddr = utilsaddr.ParseHostPortAddr

var ParseAddr = utilsaddr.ParseAddr
