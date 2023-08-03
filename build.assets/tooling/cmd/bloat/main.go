// Copyright 2023 Gravitational, Inc
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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type config struct {
	base           string
	current        string
	binaries       []string
	errorThreshold int
	warnThreshold  int
	format         string
	encoder        func(map[string]result) error
}

func main() {
	var cfg config
	flag.StringVar(&cfg.current, "current", "", "The absolute path to the build directory containing the new binaries.")
	flag.StringVar(&cfg.base, "base", "", "The absolute path to the build containing the previous binaries to compare against.")
	flag.StringVar(&cfg.format, "format", "json", "The format to display the output in. Valid values: markdown, json")
	flag.IntVar(&cfg.errorThreshold, "error-threshold", 3, "The number of MB which cannot be exceeded.")
	flag.IntVar(&cfg.errorThreshold, "warn-threshold", 1, "The number of MB which trigger a warning.")
	flag.Func("binaries", "A comma separated list of binary names to compare", func(s string) error {
		cfg.binaries = strings.Split(s, ",")
		if len(cfg.binaries) == 0 {
			return errors.New("there must be at least one binary provided to analyze via --binaries")
		}

		return nil
	})

	flag.Parse()

	if cfg.current == "" {
		fmt.Println("Must provide the new build directory via --current")
		os.Exit(1)
	}

	if cfg.base == "" {
		fmt.Println("Must provide the base build directory via --base")
		os.Exit(1)
	}

	if cfg.errorThreshold <= 0 {
		cfg.errorThreshold = 3
	}

	if cfg.warnThreshold <= 0 {
		cfg.warnThreshold = 1
	}

	if len(cfg.binaries) == 0 {
		cfg.binaries = []string{"tctl", "tsh", "teleport", "tbot"}
	}

	switch cfg.format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		cfg.encoder = func(r map[string]result) error {
			return enc.Encode(r)
		}
	case "markdown":
		enc := markdownRenderer{w: os.Stdout}
		cfg.encoder = func(r map[string]result) error {
			return enc.renderTable(r)
		}
	default:
		fmt.Printf("Uknown format %q provided\n", cfg.format)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type markdownRenderer struct {
	w io.Writer
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m markdownRenderer) renderTable(data map[string]result) error {
	titles := []string{"Binary", "Base Size", "Current Size", "Change"}

	// get the initial padding from the titles
	padding := map[string]int{}
	for _, v := range titles {
		padding[v] = len(v)
	}

	// get the largest item from the title or items in the column to determine
	// the actual padding
	for k, column := range data {
		padding["Binary"] = max(padding["Binary"], len(k))
		padding["Base Size"] = max(padding["Base Size"], len(column.BaseSize))
		padding["Current Size"] = max(padding["Current Size"], len(column.CurrentSize))
		padding["Change"] = max(padding["Change"], len(column.Change))
	}

	format := strings.Repeat("| %%-%ds ", len(padding)) + "|\n"
	paddings := []interface{}{
		padding["Binary"],
		padding["Base Size"],
		padding["Current Size"],
		padding["Change"],
	}
	format = fmt.Sprintf(format, paddings...)

	// write the heading and title
	buf := bytes.NewBufferString("# Bloat Check Results\n")
	row := []any{"Binary", "Base Size", "Current Size", "Change"}
	buf.WriteString(fmt.Sprintf(format, row...))

	// write the delimiter
	row = []interface{}{"", "", "", ""}
	buf.WriteString(strings.Replace(fmt.Sprintf(format, row...), " ", "-", -1))

	// write the rows
	for k, column := range data {
		row := []interface{}{k, column.BaseSize, column.CurrentSize, column.Change}
		buf.WriteString(fmt.Sprintf(format, row...))
	}

	_, err := m.w.Write(buf.Bytes())
	return err
}

type result struct {
	BaseSize    string `json:"base_size"`
	CurrentSize string `json:"current_size"`
	Change      string `json:"change"`
}

func run(c config) error {
	output := map[string]result{}

	var failure bool
	for _, b := range c.binaries {
		stats, err := calculateChange(c.base, c.current, b)
		if err != nil {
			return err
		}

		change := stats.currentSize - stats.baseSize
		status := "✅"
		if change > int64(c.warnThreshold) {
			status = "⚠️"
		}
		if change > int64(c.errorThreshold) {
			status = "❌"
			failure = true
		}

		baseMB := stats.baseSize / (1 << 20)
		currentMB := stats.currentSize / (1 << 20)
		output[b] = result{
			BaseSize:    fmt.Sprintf("%dMB", baseMB),
			CurrentSize: fmt.Sprintf("%dMB", currentMB),
			Change:      fmt.Sprintf("%dMB %s", currentMB-baseMB, status),
		}
	}

	if err := c.encoder(output); err != nil {
		return err
	}

	if failure {
		return errors.New("binary bloat detected - at least one binary increased by more than the allowed threshold")
	}

	return nil
}

type stats struct {
	baseSize    int64
	currentSize int64
}

func calculateChange(base, current, binary string) (stats, error) {
	baseInfo, err := os.Stat(filepath.Join(base, binary))
	if err != nil {
		return stats{}, err
	}

	currentInfo, err := os.Stat(filepath.Join(current, binary))
	if err != nil {
		return stats{}, err
	}

	return stats{
		baseSize:    baseInfo.Size(),
		currentSize: currentInfo.Size(),
	}, nil
}
