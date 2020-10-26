package web

import (
	"github.com/gravitational/teleport"

	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
)

// newPackageLogger returns a new instance of the logger
// configured for the package
func newPackageLogger(subcomponents ...string) logrus.FieldLogger {
	return logrus.WithField(trace.Component,
		teleport.Component(append([]string{teleport.ComponentWeb}, subcomponents...)...))
}
