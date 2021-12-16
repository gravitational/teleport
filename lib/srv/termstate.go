// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package srv

import (
	"math"
	"regexp"
	"strings"
)

type TermState int

const (
	TermStateShell TermState = 1
	TermStateApp   TermState = 2
)

type TermStateManager struct {
	state    TermState
	lastline []byte
	onChange func(TermState)
}

func NewTermStateManager(command string, onChange func(TermState)) *TermStateManager {
	var state TermState
	isRecognizedShell := strings.HasSuffix(command, "sh") ||
		strings.HasSuffix(command, "bash") ||
		strings.HasSuffix(command, "zsh") ||
		strings.HasSuffix(command, "fish")

	if isRecognizedShell {
		state = TermStateShell
	} else {
		state = TermStateApp
	}

	return &TermStateManager{
		state:    state,
		lastline: nil,
		onChange: onChange,
	}
}

var shellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^bash.*`),
	regexp.MustCompile(`^zsh.*`),
	regexp.MustCompile(`^sh.*`),
	regexp.MustCompile(`^fish.*`),
	regexp.MustCompile(`^.+@.+:\/#`),
}

func (g *TermStateManager) Update(data []byte) {
	startState := g.state
	defer func() {
		if g.state != startState && g.onChange != nil {
			g.onChange(g.state)
		}
	}()

	lastRet := findLast(data, '\n')
	if lastRet == math.MaxInt {
		g.lastline = append(g.lastline, data...)
	} else {
		g.lastline = data[lastRet:]
	}

	if len(g.lastline) > 0 {
		target := string(g.lastline)
		for _, pattern := range shellPatterns {
			if pattern.MatchString(target) {
				g.state = TermStateShell
				return
			}
		}
	}

	g.state = TermStateApp
}

func (g *TermStateManager) State() TermState {
	return g.state
}

func findLast(haystack []byte, needle byte) int {
	location := math.MaxInt

	for i, b := range haystack {
		if b == needle {
			location = i
		}
	}

	return location
}
