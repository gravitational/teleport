/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"flag"
	"fmt"
)

type runMode int

const (
	modeTest runMode = iota
	modeUI
	modeCodegen
	modeDebug
	modeBrowse
	modeBrowseConnect
	modeReport
	modeTestResults
	modeGitHubReport
)

func (m runMode) String() string {
	switch m {
	case modeTest:
		return "test"
	case modeUI:
		return "ui"
	case modeCodegen:
		return "codegen"
	case modeDebug:
		return "debug"
	case modeBrowse:
		return "browse"
	case modeBrowseConnect:
		return "browse-connect"
	case modeReport:
		return "report"
	case modeTestResults:
		return "test-results"
	case modeGitHubReport:
		return "github-report"
	default:
		return fmt.Sprintf("unknown(%d)", m)
	}
}

type modeOption struct {
	name    string
	usage   string
	value   runMode
	enabled bool
}

type modeSet struct {
	modes       []*modeOption
	defaultMode runMode
}

func (s *modeSet) register(name, usage string, value runMode) {
	s.modes = append(s.modes, &modeOption{
		name:  name,
		usage: usage,
		value: value,
	})
}

func (s *modeSet) bindFlags(fs *flag.FlagSet) {
	for _, m := range s.modes {
		fs.BoolVar(&m.enabled, m.name, false, m.usage)
	}
}

func (s *modeSet) resolve() (runMode, error) {
	var selected *modeOption

	for _, m := range s.modes {
		if m.enabled {
			if selected != nil {
				return 0, fmt.Errorf("--%s and --%s are mutually exclusive", selected.name, m.name)
			}

			selected = m
		}
	}

	if selected == nil {
		return s.defaultMode, nil
	}

	return selected.value, nil
}
