// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/utils"
)

func onBackendClone(ctx context.Context, configPath string) error {
	// The config flag is global on the backend command, and not required,
	// so that the default teleport config file can be used by other commands.
	// However, since clone uses its own unique config file and not the teleport
	// config file it needs to be explicitly provided.
	if configPath == "" {
		return trace.BadParameter("required flag --config/-c not provided")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	var config backend.CloneConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return trace.Wrap(err)
	}

	src, err := backend.New(ctx, config.Source.Type, config.Source.Params)
	if err != nil {
		return trace.Wrap(err, "failed to create source backend")
	}
	defer src.Close()

	dst, err := backend.New(ctx, config.Destination.Type, config.Destination.Params)
	if err != nil {
		return trace.Wrap(err, "failed to create destination backend")
	}
	defer dst.Close()

	if err := backend.Clone(ctx, src, dst, config.Parallel, config.Force); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func onBackendGet(ctx context.Context, config backend.Config, key, format string) error {
	bk, err := backend.New(ctx, config.Type, config.Params)
	if err != nil {
		return trace.Wrap(err, "creating backend")
	}

	item, err := bk.Get(ctx, backend.KeyFromString(key))
	if err != nil {
		return trace.Wrap(err, "getting item")
	}

	return trace.Wrap(printBackendItems(format, stream.Once(*item)))
}

func onBackendList(ctx context.Context, config backend.Config, prefix, format string) error {
	bk, err := backend.New(ctx, config.Type, config.Params)
	if err != nil {
		return trace.Wrap(err, "creating backend")
	}

	var startKey, endKey backend.Key
	if prefix != "" {
		startKey = backend.KeyFromString(prefix)
		endKey = backend.RangeEnd(startKey.ExactKey())
	} else {
		startKey = backend.NewKey("")
		endKey = backend.RangeEnd(startKey)
	}

	items := bk.Items(ctx, backend.ItemsParams{StartKey: startKey, EndKey: endKey})

	return trace.Wrap(printBackendItems(format, items))
}

func printBackendItems(format string, items iter.Seq2[backend.Item, error]) error {
	// displayItem exists to better represent a
	// [backend.Item] to users. The Key is converted to
	// it's textual representation, and the Value is
	// converted to a string so that it is not base64 encoded
	// during marshaling.
	type displayItem struct {
		Key      string
		Value    string
		Revision string
		Expires  time.Time
	}

	backendItems := stream.FilterMap(items, func(i backend.Item) (displayItem, bool) {
		return displayItem{
			Key:      i.Key.String(),
			Expires:  i.Expires,
			Revision: i.Revision,
			Value:    string(i.Value),
		}, true
	})

	switch strings.ToLower(format) {
	case teleport.Text, "":
		table := asciitable.MakeTable([]string{"Key", "Expires", "Revision"})
		for row, err := range stream.FilterMap(backendItems, func(i displayItem) ([]string, bool) {
			expiry := "Never"
			if !i.Expires.IsZero() {
				expiry = i.Expires.Format(time.RFC3339)
			}
			return []string{i.Key, expiry, i.Revision}, true
		}) {
			if err != nil {
				return trace.Wrap(err, "retrieving items")
			}

			table.AddRow(row)
		}

		if err := table.WriteTo(os.Stdout); err != nil {
			return trace.Wrap(err)
		}
	case teleport.JSON:
		allItems, err := stream.Collect(backendItems)
		if err != nil {
			return trace.Wrap(err, "collecting items")
		}

		out, err := utils.FastMarshalIndent(allItems, "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := fmt.Fprintln(os.Stdout, string(out)); err != nil {
			return trace.Wrap(err)
		}
	case teleport.YAML:
		allItems, err := stream.Collect(backendItems)
		if err != nil {
			return trace.Wrap(err, "collecting items")
		}

		out, err := yaml.Marshal(allItems)
		if err != nil {
			return trace.Wrap(err)
		}

		if _, err := fmt.Fprintln(os.Stdout, string(out)); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported format %q", format)
	}

	return nil
}

func onBackendDelete(ctx context.Context, config backend.Config, key string) error {
	// logs are discarded to prevent any logging output
	// from stomping on the prompt.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	bk, err := backend.New(ctx, config.Type, config.Params)
	if err != nil {
		return trace.Wrap(err, "creating backend")
	}

	ok, err := prompt.Confirmation(ctx, os.Stdout, prompt.Stdin(), fmt.Sprintf("Are you sure you want to delete %s", key))
	if err != nil {
		return trace.Wrap(err)
	} else if !ok {
		return trace.Errorf("Operation canceled by user request.")
	}

	if err := bk.Delete(ctx, backend.KeyFromString(key)); err != nil {
		return trace.Wrap(err, "deleting item")
	}

	fmt.Printf("item %q has been deleted\n", key)
	return nil
}

func checksum(filename string) (string, error) {
	f, err := utils.OpenFileAllowingUnsafeLinks(filename)
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", trace.Wrap(err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func onBackendEdit(ctx context.Context, config backend.Config, key string) error {
	// logs are discarded to prevent any logging output
	// from being written over the editor.
	slog.SetDefault(slog.New(slog.DiscardHandler))

	bk, err := backend.New(ctx, config.Type, config.Params)
	if err != nil {
		return trace.Wrap(err, "creating backend")
	}

	item, err := bk.Get(ctx, backend.KeyFromString(key))
	if err != nil {
		return trace.Wrap(err, "getting item")
	}

	f, err := os.CreateTemp("", "teleport-item*.yaml")
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := os.Remove(f.Name()); err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: could not remove temporary file %v\n", f.Name())
		}
	}()

	// displayItem exists to better represent a
	// [backend.Item] to users when editing. The
	// Key is omitted as that isn't allowed to be modified,
	// and the Value is converted to a string so that it
	// is not base64 encoded during marshaling.
	type displayItem struct {
		Revision string
		Expires  time.Time
		Value    string
	}
	if err := utils.WriteYAML(f, displayItem{
		Revision: item.Revision,
		Expires:  item.Expires,
		Value:    string(item.Value),
	}); err != nil {
		return trace.Wrap(err)
	}

	if err := f.Close(); err != nil {
		return trace.Wrap(err)
	}

	originalSum, err := checksum(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}

	editor := cmp.Or(os.Getenv("TELEPORT_EDITOR"), os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
	args := strings.Fields(editor)
	editorCmd := exec.CommandContext(ctx, args[0], append(args[1:], f.Name())...)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Start(); err != nil {
		return trace.BadParameter("could not start editor %v: %v", editor, err)
	}

	if err := editorCmd.Wait(); err != nil {
		return trace.BadParameter("skipping resource update, editor did not complete successfully: %v", err)
	}

	newSum, err := checksum(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}

	// nothing to do if the resource was not modified
	if newSum == originalSum {
		fmt.Println("edit canceled, no changes made")
		return nil
	}

	editedFile, err := utils.OpenFileAllowingUnsafeLinks(f.Name())
	if err != nil {
		return trace.Wrap(err)
	}
	defer editedFile.Close()

	decoder := kyaml.NewYAMLOrJSONDecoder(editedFile, defaults.LookaheadBufSize)

	var editedItem displayItem
	if err := decoder.Decode(&editedItem); err != nil {
		if errors.Is(err, io.EOF) {
			return trace.BadParameter("no item found, empty input?")
		}
		return trace.Wrap(err)
	}

	// The Key is not presented to the users and is
	// not allowed to be modified. All other values
	// can be adjusted, though due to the use of
	// ConditionalUpdate below tweaking the revision
	// will prevent updates unless it's being changed
	// to the correct value.
	item.Revision = editedItem.Revision
	item.Expires = editedItem.Expires
	item.Value = []byte(editedItem.Value)

	if _, err := bk.ConditionalUpdate(ctx, *item); err != nil {
		return trace.Wrap(err, "updating item in backend")
	}

	return nil
}
