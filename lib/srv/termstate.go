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
	"sync"

	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type termState int

const (
	TermStateShell termState = 1
	TermStateApp   termState = 2
)

type TermManager struct {
	state    termState
	lastline []byte
	W        *kubeutils.SwitchWriter
	messages []string
	mu       sync.Mutex
}

func NewTermManager(command string, w *kubeutils.SwitchWriter) *TermManager {
	var state termState
	isRecognizedShell := strings.HasSuffix(command, "sh") ||
		strings.HasSuffix(command, "bash") ||
		strings.HasSuffix(command, "zsh") ||
		strings.HasSuffix(command, "fish") ||
		len(command) == 0

	if isRecognizedShell {
		state = TermStateShell
	} else {
		state = TermStateApp
	}

	return &TermManager{
		state:    state,
		lastline: nil,
		W:        w,
	}
}

func (g *TermManager) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	count, err := g.W.Write(p)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	preState := g.state
	g.update(p[:count])
	if preState == TermStateApp && g.state == TermStateShell {
		err := g.flushMessages()
		if err != nil {
			return 0, trace.Wrap(err)
		}
	}

	return count, nil
}

func (g *TermManager) flushMessages() error {
	for _, message := range g.messages {
		err := utils.WriteAll(g.W.Write, []byte(message))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	g.messages = nil
	return nil
}

func (g *TermManager) BroadcastMessage(message string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.state == TermStateShell {
		err := utils.WriteAll(g.W.Write, []byte(message))
		return trace.Wrap(err)
	}

	g.messages = append(g.messages, message)
	return nil
}

var shellPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^bash.*`),
	regexp.MustCompile(`^zsh.*`),
	regexp.MustCompile(`^sh.*`),
	regexp.MustCompile(`^fish.*`),
	regexp.MustCompile(`^.+@.+:\/#`),
}

func (g *TermManager) update(data []byte) {
	lastRet := findLast(data, '\n')
	if lastRet == math.MaxInt {
		g.lastline = append(g.lastline, data...)
	} else {
		g.lastline = data[lastRet:]
	}

	target := string(g.lastline)
	if strings.HasPrefix("Teleport > ", string(g.lastline)) {
		g.state = TermStateShell
		return
	}

	if len(g.lastline) > 0 {
		for _, pattern := range shellPatterns {
			if pattern.MatchString(target) {
				g.state = TermStateShell
				return
			}
		}
	}

	g.state = TermStateApp
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
