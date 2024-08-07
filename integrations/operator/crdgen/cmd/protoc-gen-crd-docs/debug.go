//go:build debug

/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"os"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	crdgen "github.com/gravitational/teleport/integrations/operator/crdgen/lib"
)

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)

	inputPath := os.Getenv(crdgen.PluginInputPathEnvironment)
	if inputPath == "" {
		log.Error(
			trace.BadParameter(
				"When built with the 'debug' tag, the input path must be set through the environment variable: %s",
				crdgen.PluginInputPathEnvironment,
			),
		)
		os.Exit(-1)
	}
	log.Infof("This is a debug build, the protoc request is read from the file: '%s'", inputPath)

	req, err := crdgen.ReadRequestFromFile(inputPath)
	if err != nil {
		log.WithError(err).Error("error reading request from file")
		os.Exit(-1)
	}

	if err := crdgen.HandleDocsRequest(req); err != nil {
		log.WithError(err).Error("Failed to generate docs")
		os.Exit(-1)
	}
}
