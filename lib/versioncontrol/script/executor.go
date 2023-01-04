/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package script

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	vc "github.com/gravitational/teleport/api/versioncontrol"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	log "github.com/sirupsen/logrus"
)

const (
	fileParams = "params.json"
	fileResult = "result.json"
	fileScript = "script.sh"
	fileOutput = "output.log"
)

const fileMode os.FileMode = 0600

const createOpts int = os.O_WRONLY | os.O_CREATE | os.O_TRUNC

// ExecutorConfig configures a script executor.
type ExecutorConfig struct {
	// Current represents the currently running teleport build. Some exec messages
	// include assertions that require specific parameters.
	Current vc.Target
	// Dir is the directory under which all execs occur.
	Dir string
	// Shell is the default shell.
	Shell string
	// Clock is used to for timestamps and ttls.
	Clock clockwork.Clock
}

// Executor is a helper for managing script execution. In practice, this type doesn't do much except
// manage a standardized directory layout.
type Executor struct {
	cfg ExecutorConfig

	// mu protects internal state for cleanup operations
	mu sync.Mutex

	dangling []Ref
}

func NewExecutor(cfg ExecutorConfig) (*Executor, error) {
	if cfg.Dir == "" {
		return nil, trace.BadParameter("missing required parameter 'Dir' for script executor")
	}

	if cfg.Shell == "" {
		cfg.Shell = defaults.DefaultShell
	}

	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}

	return &Executor{
		cfg: cfg,
	}, nil
}

// ListEntries lists all entries in this executor's cache dir.
func (e *Executor) ListEntries() ([]Ref, error) {
	entries, err := os.ReadDir(e.cfg.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, trace.Errorf("failed to read exec dir %q: %v", e.cfg.Dir, err)
	}

	var refs []Ref

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		ref, ok := parseRef(entry.Name())
		if !ok {
			continue
		}

		refs = append(refs, ref)
	}

	return refs, nil
}

func (e *Executor) ExpireEntries(ttl time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	entries, err := e.ListEntries()
	if err != nil {
		return trace.Wrap(err)
	}

	var dangling []Ref

	for _, ref := range entries {
		params, err := e.LoadParams(ref)
		if err != nil {
			// failure to load params either means that this entry was just created, or that
			// it is corrupted/malformed. if it was dangling on the last pass as well, it will
			// be removed.
			dangling = append(dangling, ref)
		}

		// note that we're calculating expiry time based on the 'params' time, not the 'result' time. this is
		// because we can't guarantee that 'result.json' is ever written.
		if !params.Time.Add(ttl).After(e.cfg.Clock.Now()) {
			if err := e.clear(ref); err != nil {
				log.Warnf("Failed to clear expired exec entry %s: %v", ref, err)
			}
		}
	}

	// entries that were observed to be dangling on the previous pass are
	// assumed to be permanently malformed and are removed. new dangling entries
	// are preserved so that they can be checked again later.
	filtered := dangling[:0]
Outer:
	for _, ref := range dangling {
		for _, prev := range e.dangling {
			if prev == ref {
				if err := e.clear(ref); err != nil {
					log.Warnf("Failed to clear dangling exec entry %s: %v", ref, err)
				}
				continue Outer
			}
		}
		filtered = append(filtered, ref)
	}
	e.dangling = filtered

	return nil
}

func (e *Executor) Exec(params types.ExecScript) types.ExecScriptResult {
	if params.Shell == "" {
		params.Shell = e.cfg.Shell
	}

	if params.Time.IsZero() {
		params.Time = e.cfg.Clock.Now().UTC()
	}

	if err := params.Check(); err != nil {
		return types.ExecScriptResult{
			Type:  params.Type,
			ID:    params.ID,
			Time:  e.cfg.Clock.Now().UTC(),
			Error: err.Error(),
		}
	}

	// check the 'Expect' target attributes against our 'Current' target. typically
	// this just means checking that our current teleport version matches the teleport
	// versio that the exec msg was crafted for.
	for key, val := range params.Expect {
		if e.cfg.Current[key] != val {
			return types.ExecScriptResult{
				Type:  params.Type,
				ID:    params.ID,
				Time:  e.cfg.Clock.Now().UTC(),
				Error: fmt.Sprintf("exec expects target attr %q to be %q, got %q", key, val, e.cfg.Current[key]),
			}
		}
	}

	dir, err := e.dirPath(Ref{
		Type: params.Type,
		ID:   params.ID,
	})

	if err != nil {
		return types.ExecScriptResult{
			Type:  params.Type,
			ID:    params.ID,
			Time:  e.cfg.Clock.Now().UTC(),
			Error: err.Error(),
		}
	}

	exec := execution{
		params: params,
		dir:    dir,
	}

	if err := exec.init(); err != nil {
		log.Warnf("ExecScript %s-%d init failed: %v", params.Type, params.ID, err)
		return types.ExecScriptResult{
			Type:  params.Type,
			ID:    params.ID,
			Time:  e.cfg.Clock.Now().UTC(),
			Error: err.Error(),
		}
	}

	state, err := exec.run()

	result := types.ExecScriptResult{
		Type: params.Type,
		ID:   params.ID,
		Time: e.cfg.Clock.Now().UTC(),
	}

	if err != nil {
		log.Warnf("ExecScript %s-%d run failed: %v", params.Type, params.ID, err)
		result.Error = err.Error()
	}
	if state != nil {
		result.Success = state.Success()
		result.Code = int32(state.ExitCode())
	}

	if err := exec.writeJSON(fileResult, result); err != nil {
		log.Warnf("ExecScript %s-%d result write failed: %v", params.Type, params.ID, err)
	}

	return result
}

func (e *Executor) dirPath(ref Ref) (string, error) {
	if !types.IsStrictKebabCase(ref.Type) {
		// we're extra stringent about exec type name since it is
		// used as dir name.
		return "", trace.BadParameter("invalid exec type %q", ref.Type)
	}

	return filepath.Join(e.cfg.Dir, ref.String()), nil
}

func (e *Executor) LoadParams(ref Ref) (types.ExecScript, error) {
	dir, err := e.dirPath(ref)
	if err != nil {
		return types.ExecScript{}, trace.Wrap(err)
	}
	exec := execution{
		dir: dir,
	}

	var val types.ExecScript
	return val, exec.readJSON(fileParams, &val)
}

func (e *Executor) LoadResult(ref Ref) (types.ExecScriptResult, error) {
	dir, err := e.dirPath(ref)
	if err != nil {
		return types.ExecScriptResult{}, trace.Wrap(err)
	}
	exec := execution{
		dir: dir,
	}

	var val types.ExecScriptResult
	return val, exec.readJSON(fileResult, &val)
}

func (e *Executor) LoadOutput(ref Ref) (string, error) {
	dir, err := e.dirPath(ref)
	if err != nil {
		return "", trace.Wrap(err)
	}
	exec := execution{
		dir: dir,
	}

	return exec.readString(fileOutput)
}

func (e *Executor) clear(ref Ref) error {
	dir, err := e.dirPath(ref)
	if err != nil {
		return trace.Wrap(err)
	}
	exec := execution{
		dir: dir,
	}

	return exec.clear()
}

// Ref is a reference to a unique execution.
type Ref struct {
	Type string
	ID   uint64
}

func (r Ref) String() string {
	return fmt.Sprintf("%s-%d", r.Type, r.ID)
}

// parseRef attempts to decode a ref value from a string.
func parseRef(s string) (r Ref, ok bool) {
	i := strings.LastIndex(s, "-")
	if i < 1 {
		return Ref{}, false
	}

	rtype, sid := s[:i], s[i+1:]
	id, err := strconv.ParseUint(sid, 10, 64)
	return Ref{rtype, id}, err == nil
}

// execution is a helper used to interact with the files of a specific execution attempt.
type execution struct {
	params types.ExecScript
	dir    string
}

func (e *execution) path(file string) string {
	return filepath.Join(e.dir, file)
}

func (e *execution) writeJSON(file string, value any) error {
	path := e.path(file)
	f, err := os.OpenFile(path, createOpts, fileMode)
	if err != nil {
		return trace.Errorf("failed to create file %q: %v", path, err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(value); err != nil {
		return trace.Errorf("failed to encode file %q: %v", path, err)
	}

	if err := f.Sync(); err != nil {
		return trace.Errorf("failed to flush file %q: %v", path, err)
	}
	return nil
}

func (e *execution) readJSON(file string, value any) error {
	path := e.path(file)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return trace.NotFound("failed to locate file %q", path)
		}
		return trace.Errorf("failed to open file %q: %v", path, err)
	}

	if err := json.NewDecoder(f).Decode(value); err != nil {
		return trace.Errorf("failed to decode file %q: %v", path, err)
	}

	return nil
}

func (e *execution) readString(file string) (string, error) {
	path := e.path(file)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", trace.NotFound("failed to locate file %q", path)
		}
		return "", trace.Errorf("failed to open file %q: %v", path, err)
	}

	s, err := io.ReadAll(f)
	if err != nil {
		return "", trace.Errorf("failed to load file %q: %v", path, err)
	}

	return string(s), nil
}

func (e *execution) run() (*os.ProcessState, error) {
	outPath := e.path(fileOutput)
	out, err := os.OpenFile(outPath, createOpts, fileMode)
	if err != nil {
		return nil, trace.Errorf("failed to create output file %q: %v", outPath, err)
	}
	defer out.Close()

	// we approximate the behavior of a shebang by allowing a single optional space separated argument after
	// the path to the interpreter. Commands will take one of two possible forms: '<cmd> <script>' or
	// '<cmd> <arg> <script>'. the main reason to do this is to support the common `/usr/bin/env <interpreter>`
	// pattern.
	parts := strings.SplitN(strings.TrimSpace(e.params.Shell), " ", 2)
	parts = append(parts, e.path(fileScript))

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = out
	cmd.Stderr = out

	cmd.Dir = e.dir

	// ensure env is non-nil even if no vars are specified (prevents unexpected inheritance).
	cmd.Env = []string{}

	for key, val := range e.params.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
	}

	for _, key := range e.params.EnvPassthrough {
		val := os.Getenv(key)
		if val == "" {
			continue
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, val))
	}

	if err := cmd.Start(); err != nil {
		return nil, trace.Errorf("cmd failed to start: %v", err)
	}

	err = cmd.Wait()

	if err != nil {
		err = trace.Errorf("error while running: %v", err)
	}

	return cmd.ProcessState, err
}

// init sets up the exec dir, writing the params.json and script.sh files.
func (e *execution) init() error {
	if err := e.clear(); err != nil {
		return trace.Wrap(err)
	}

	// set of the directory for this execution.
	if err := os.Mkdir(e.dir, teleport.SharedDirMode); err != nil {
		return trace.Errorf("failed to create exec dir %q: %v", e.dir, err)
	}

	// temporarily clear the script field. we store script value separately.
	script := e.params.Script
	e.params.Script = ""
	defer func() {
		e.params.Script = script
	}()

	if err := e.writeJSON(fileParams, e.params); err != nil {
		return trace.Wrap(err)
	}

	// set up the script.sh file
	sfPath := e.path(fileScript)
	// note that we don't bother marking the script as executable, since it is passed
	// to its interpreter explicitly.
	sf, err := os.OpenFile(sfPath, createOpts, fileMode)
	if err != nil {
		return trace.Errorf("failed to create params file %q: %v", sfPath, err)
	}
	defer sf.Close()

	if _, err := sf.WriteString(script); err != nil {
		return trace.Errorf("failed to write script file %q: %v", sfPath, err)
	}

	if err := sf.Sync(); err != nil {
		return trace.Errorf("failed to flush script file %q: %v", sfPath, err)
	}

	return nil
}

func (e *execution) clear() error {
	err := os.RemoveAll(e.dir)
	if err != nil && err != os.ErrNotExist {
		return trace.Errorf("failed to clear exec dir %q: %v", e.dir, err)
	}
	return nil
}
