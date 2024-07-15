/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/tbot/botfs"
	"github.com/gravitational/teleport/lib/utils"
)

const DestinationDirectoryType = "directory"

// DestinationDirectory is a Destination that writes to the local filesystem
type DestinationDirectory struct {
	Path     string             `yaml:"path,omitempty"`
	Symlinks botfs.SymlinksMode `yaml:"symlinks,omitempty"`
	ACLs     botfs.ACLMode      `yaml:"acls,omitempty"`
}

func (dd *DestinationDirectory) UnmarshalYAML(node *yaml.Node) error {
	// Accept either a string path or a full struct (allowing for options in
	// the future, e.g. configuring permissions, etc):
	//   directory: /foo
	// or:
	//   directory:
	//     path: /foo
	//     some_future_option: bar

	var path string
	if err := node.Decode(&path); err == nil {
		dd.Path = path
		return nil
	}

	// Shenanigans to prevent UnmarshalYAML from recursing back to this
	// override (we want to use standard unmarshal behavior for the full
	// struct)
	type rawDirectory DestinationDirectory
	return trace.Wrap(node.Decode((*rawDirectory)(dd)))
}

func (dd *DestinationDirectory) CheckAndSetDefaults() error {
	if dd.Path == "" {
		return trace.BadParameter("destination path must not be empty")
	}

	switch dd.Symlinks {
	case "":
		// We default to SymlinksTrySecure. It's become apparent that the
		// kernel version alone is not usually enough information to know that
		// secure symlinks is supported by the OS. In future, we should aim to
		// perform a more definitive test.
		dd.Symlinks = botfs.SymlinksTrySecure
	case botfs.SymlinksInsecure, botfs.SymlinksTrySecure:
		// valid
	case botfs.SymlinksSecure:
		if !botfs.HasSecureWriteSupport() {
			return trace.BadParameter("symlink mode %q not supported on this system", dd.Symlinks)
		}
	default:
		return trace.BadParameter("invalid symlinks mode: %q", dd.Symlinks)
	}

	aclsSupported := botfs.HasACLSupport()
	switch dd.ACLs {
	case "":
		if aclsSupported {
			// Unlike openat2(), we can't ever depend on ACLs being available.
			// We'll only ever try to use them, end users can opt-in to a hard
			// ACL check if they wish.
			dd.ACLs = botfs.ACLTry
		} else {
			// if aclsSupported == false here, we know it will never work, so
			// don't bother trying.
			dd.ACLs = botfs.ACLOff
		}
	case botfs.ACLOff, botfs.ACLTry:
		// valid
	case botfs.ACLRequired:
		if !aclsSupported {
			return trace.BadParameter("acls mode %q not supported on this system", dd.ACLs)
		}
	default:
		return trace.BadParameter("invalid acls mode: %q", dd.ACLs)
	}

	return nil
}

// mkdir attempts to make the given directory with extra logging.
func mkdir(p string) error {
	stat, err := os.Stat(p)
	if trace.IsNotFound(err) {
		if err := os.MkdirAll(p, botfs.DefaultDirMode); err != nil {
			if errors.Is(err, fs.ErrPermission) {
				return trace.Wrap(err, "Teleport does not have permission to write to %v. Ensure that you are running as a user with appropriate permissions.", p)
			}
			return trace.Wrap(err)
		}

		log.InfoContext(
			context.TODO(),
			"Created directory",
			"path", p,
		)
	} else if err != nil {
		// this can occur if we are unable to read the data dir
		if errors.Is(err, fs.ErrPermission) {
			return trace.Wrap(err, "Teleport does not have permission to access: %v. Ensure that you are running as a user with appropriate permissions.", p)
		}
		return trace.Wrap(err)
	} else if !stat.IsDir() {
		return trace.BadParameter("Path %q already exists and is not a directory", p)
	}

	return nil
}

func (dd *DestinationDirectory) Init(_ context.Context, subdirs []string) error {
	// Create the directory if needed.
	if err := mkdir(dd.Path); err != nil {
		return trace.Wrap(err)
	}

	for _, dir := range subdirs {
		if err := mkdir(path.Join(dd.Path, dir)); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (dd *DestinationDirectory) Verify(keys []string) error {
	currentUser, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}

	stat, err := os.Stat(dd.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	ownedByBot, err := botfs.IsOwnedBy(stat, currentUser)
	if trace.IsNotImplemented(err) {
		// If file owners aren't supported, ACLs certainly aren't. Just bail.
		// (Subject to change if we ever try to support Windows ACLs.)
		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	// Make sure it's worth warning about ACLs for this Destination. If ACLs
	// are disabled, unsupported, or the Destination is owned by the bot
	// (implying the user is not trying to use ACLs), just bail.
	if dd.ACLs == botfs.ACLOff || !botfs.HasACLSupport() || ownedByBot {
		return nil
	}

	errors := []error{}
	for _, key := range keys {
		path := filepath.Join(dd.Path, key)

		errors = append(errors, botfs.VerifyACL(path, &botfs.ACLOptions{
			BotUser: currentUser,
		}))
	}

	aggregate := trace.NewAggregate(errors...)
	if dd.ACLs == botfs.ACLRequired {
		// Hard fail if ACLs are specifically requested and there are errors.
		return aggregate
	}

	if aggregate != nil {
		log.WarnContext(
			context.TODO(),
			"Destination has unexpected ACLs",
			"path", dd.Path,
			"errors", aggregate,
		)
	}

	return nil
}

func (dd *DestinationDirectory) Write(ctx context.Context, name string, data []byte) error {
	_, span := tracer.Start(
		ctx,
		"DestinationDirectory/Write",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	return trace.Wrap(botfs.Write(filepath.Join(dd.Path, name), data, dd.Symlinks))
}

func (dd *DestinationDirectory) Read(ctx context.Context, name string) ([]byte, error) {
	_, span := tracer.Start(
		ctx,
		"DestinationDirectory/Read",
		oteltrace.WithAttributes(attribute.String("name", name)),
	)
	defer span.End()

	data, err := botfs.Read(filepath.Join(dd.Path, name), dd.Symlinks)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return data, nil
}

func (dd *DestinationDirectory) String() string {
	return fmt.Sprintf("%s: %s", DestinationDirectoryType, dd.Path)
}

func (dd *DestinationDirectory) TryLock() (func() error, error) {
	// TryLock should only be used for bot data directory and not for
	// destinations until an investigation on how locks will play with
	// ACLs has been completed.
	unlock, err := utils.FSTryWriteLock(filepath.Join(dd.Path, "lock"))
	return unlock, trace.Wrap(err)
}

func (dm *DestinationDirectory) MarshalYAML() (interface{}, error) {
	type raw DestinationDirectory
	return withTypeHeader((*raw)(dm), DestinationDirectoryType)
}
