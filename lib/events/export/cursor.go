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

package export

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

const (
	// completedName is the completed file name
	completedName = "completed-chunks"

	// chunkSuffix is the suffix for per-chunk cursor files
	chunkSuffix = ".chunk"
)

// CursorConfig configures a cursor.
type CursorConfig struct {
	// Dir is the cursor directory. This directory will be created if it does not exist
	// and should not be used for any other purpose.
	Dir string
}

// CheckAndSetDefaults validates configuration and sets default values for optional parameters.
func (c *CursorConfig) CheckAndSetDefaults() error {
	if c.Dir == "" {
		return trace.BadParameter("missing required parameter Dir in CursorConfig")
	}

	return nil
}

// Cursor manages an event export cursor directory and keeps a copy of its state in-memory,
// improving the efficiency of updates by only writing diffs to disk. the cursor directory
// contains a sub-directory per date. each date's state is tracked using an append-only list
// of completed chunks, along with a per-chunk cursor file. cursor directories are not intended
// for true concurrent use, but concurrent use in the context of a graceful restart won't have
// any consequences more dire than duplicate events.
type Cursor struct {
	cfg   CursorConfig
	mu    sync.Mutex
	state ExporterState
}

// NewCursor creates a new cursor, loading any preexisting state from disk.
func NewCursor(cfg CursorConfig) (*Cursor, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	state, err := loadInitialState(cfg.Dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Cursor{
		cfg:   cfg,
		state: *state,
	}, nil
}

// GetState gets the current state as seen by this cursor.
func (c *Cursor) GetState() ExporterState {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.state.Clone()
}

// Sync synchronizes the cursor's in-memory state with the provided state, writing any diffs to disk.
func (c *Cursor) Sync(newState ExporterState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for d, s := range newState.Dates {
		if err := c.syncDate(d, s); err != nil {
			return trace.Wrap(err)
		}
	}

	for d := range c.state.Dates {
		if _, ok := newState.Dates[d]; !ok {
			if err := c.deleteDate(d); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

func (c *Cursor) syncDate(date time.Time, state DateExporterState) error {
	// ensure date directory exists. the existence of the date directory
	// is meaningful even if it contains no files.
	dateDir := filepath.Join(c.cfg.Dir, date.Format(time.DateOnly))
	if err := os.MkdirAll(dateDir, teleport.SharedDirMode); err != nil {
		return trace.ConvertSystemError(err)
	}

	// open completed file in append mode
	completedFile, err := os.OpenFile(filepath.Join(dateDir, completedName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer completedFile.Close()

	current, ok := c.state.Dates[date]
	if !ok {
		current = DateExporterState{
			Cursors: make(map[string]string),
		}
	}
	defer func() {
		c.state.Dates[date] = current
	}()

	for _, chunk := range state.Completed {
		if slices.Contains(current.Completed, chunk) {
			// already written to disk
			continue
		}

		// add chunk to completed file
		if _, err := fmt.Fprintln(completedFile, chunk); err != nil {
			return trace.ConvertSystemError(err)
		}

		// ensure chunk is flushed to disk successfully before removing the cursor file
		// and updating in-memory state.
		if err := completedFile.Sync(); err != nil {
			return trace.ConvertSystemError(err)
		}

		// delete cursor file if it exists
		if err := os.Remove(filepath.Join(dateDir, chunk+chunkSuffix)); err != nil && !os.IsNotExist(err) {
			return trace.ConvertSystemError(err)
		}

		// update current state
		current.Completed = append(current.Completed, chunk)
		delete(current.Cursors, chunk)
	}

	for chunk, cursor := range state.Cursors {
		if current.Cursors[chunk] == cursor {
			continue
		}

		// write cursor file
		if err := os.WriteFile(filepath.Join(dateDir, chunk+chunkSuffix), []byte(cursor), 0644); err != nil {
			return trace.ConvertSystemError(err)
		}

		// update current state
		current.Cursors[chunk] = cursor
	}

	return nil
}

func (c *Cursor) deleteDate(date time.Time) error {
	if _, ok := c.state.Dates[date]; !ok {
		return nil
	}

	// delete the date directory and all its contents
	if err := os.RemoveAll(filepath.Join(c.cfg.Dir, date.Format(time.DateOnly))); err != nil {
		return trace.ConvertSystemError(err)
	}

	delete(c.state.Dates, date)

	return nil
}

func loadInitialState(dir string) (*ExporterState, error) {
	state := ExporterState{
		Dates: make(map[time.Time]DateExporterState),
	}
	// list subdirectories of the cursors v2 directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return &state, nil
		}
		return nil, trace.ConvertSystemError(err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			// ignore non-directories
			continue
		}

		// attempt to parse dir name as date
		date, err := time.Parse(time.DateOnly, entry.Name())
		if err != nil {
			// ignore non-date directories
			continue
		}

		dateState := DateExporterState{
			Cursors: make(map[string]string),
		}

		dateEntries, err := os.ReadDir(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}

		for _, dateEntry := range dateEntries {
			if dateEntry.IsDir() {
				continue
			}

			if dateEntry.Name() == completedName {
				// load the completed file
				b, err := os.ReadFile(filepath.Join(dir, entry.Name(), completedName))
				if err != nil {
					return nil, trace.ConvertSystemError(err)
				}

				// split the completed file into whitespace-separated chunks
				dateState.Completed = strings.Fields(string(b))
				continue
			}

			if !strings.HasSuffix(dateEntry.Name(), chunkSuffix) {
				continue
			}

			chunk := strings.TrimSuffix(dateEntry.Name(), chunkSuffix)
			b, err := os.ReadFile(filepath.Join(dir, entry.Name(), dateEntry.Name()))
			if err != nil {
				return nil, trace.ConvertSystemError(err)
			}

			if cc := bytes.TrimSpace(b); len(cc) != 0 {
				dateState.Cursors[chunk] = string(cc)
			}
		}

		// note that some dates may not contain any chunks. we still want to track the
		// fact that these dates have had their dirs initialized since that still indicates
		// how far we've gotten in the export process.
		state.Dates[date] = dateState
	}

	return &state, nil
}
