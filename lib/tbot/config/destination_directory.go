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
	"strings"

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
	Path     string               `yaml:"path,omitempty"`
	Symlinks botfs.SymlinksMode   `yaml:"symlinks,omitempty"`
	ACLs     botfs.ACLMode        `yaml:"acls,omitempty"`
	Readers  []*botfs.ACLSelector `yaml:"readers,omitempty"`

	// aclsEnabled is set during `Init()` if new-style ACLs are requested and
	// the ACL test succeeds. When true, ACLs will be corrected on-the-fly
	// during `Write()`.
	aclsEnabled bool
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

	for i, reader := range dd.Readers {
		if err := reader.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err, "reader entry %d is invalid", i)
		}
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

func (dd *DestinationDirectory) Init(ctx context.Context, subdirs []string) error {
	// Create the directory if needed.
	if err := mkdir(dd.Path); err != nil {
		return trace.Wrap(err)
	}

	// Check for ACL configuration and test to ensure support.
	if len(dd.Readers) > 0 && dd.ACLs != botfs.ACLOff {
		// Run a test to ensure we can actually make use of them.
		if err := botfs.TestACL(dd.Path, dd.Readers); err != nil {
			if dd.ACLs == botfs.ACLRequired {
				return trace.Wrap(err, "ACLs were marked as required but the "+
					"write test failed. Remove `acls: required` or resolve the "+
					"underlying issue.")
			}

			const msg = "ACLs were requested but could not be configured, they " +
				"will be disabled. To resolve this warning, ensure ACLs are " +
				"supported and the directory is owned by the bot user, or " +
				"remove any configured readers to disable ACLs"
			log.WarnContext(
				ctx,
				msg,
				"path", dd.Path,
				"error", err,
				"readers", dd.Readers,
			)

			dd.aclsEnabled = false
		} else {
			dd.aclsEnabled = true
			log.InfoContext(
				ctx,
				"ACL test succeeded and will be configured at runtime",
				"path", dd.Path,
				"readers", len(dd.Readers),
			)
		}
	}

	if dd.aclsEnabled {
		// Correct the base directory ACLs. Note that no default ACL is
		// configured; we manage each file/subdirectory ACL manually.
		if err := dd.verifyAndCorrectACL(ctx, ""); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, dir := range subdirs {
		if err := mkdir(path.Join(dd.Path, dir)); err != nil {
			return trace.Wrap(err)
		}

		if dd.aclsEnabled {
			if err := dd.verifyAndCorrectACL(ctx, dir); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// verifyLegacyACLs performs minor runtime verification of legacy-style ACLs,
// where it is _not_ assumed that the destination is owned by the bot user.
// This will not attempt to correct any issues, but will cause a hard failure if
// `acls: required` is configured and issues are detected.
func (dd *DestinationDirectory) verifyLegacyACLs(keys []string) error {
	currentUser, err := user.Current()
	if err != nil {
		// user.Current will fail if the user id does not exist in /etc/passwd
		// as is the case with some containerized environments.
		// TODO(noah): Switch to os.Getuid / handling UIDs directly.
		if dd.ACLs == botfs.ACLRequired {
			return trace.Wrap(err, "determining current user")
		}
		log.WarnContext(
			context.TODO(),
			"Unable to determine current user, ACLs will not be checked. To silence this warning, set ACL mode to `off`.",
			"path", dd.Path,
			"error", err,
		)
		return nil
	}

	errors := []error{}
	for _, key := range keys {
		path := filepath.Join(dd.Path, key)

		errors = append(errors, botfs.VerifyLegacyACL(path, &botfs.ACLOptions{
			BotUser: currentUser,
		}))
	}

	// Unlike new-style ACLs, we'll allow hard fails here: we don't expect to
	// be able to manage legacy ACLs at runtime and need to depend on them being
	// correct from the start, so hard fails at this stage make sense.
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

func (dd *DestinationDirectory) Verify(keys []string) error {
	// Preflight: Warn on common misconfigurations where any kind of ACL
	// management will be impossible.
	if dd.ACLs == botfs.ACLOff {
		if len(dd.Readers) > 0 {
			log.InfoContext(
				context.TODO(),
				"Readers are configured but ACLs are disabled for this destination. No ACLs will be managed or verified.",
				"path", dd.Path,
			)
		}
		return nil
	}

	if !botfs.HasACLSupport() {
		if dd.ACLs == botfs.ACLRequired {
			return trace.BadParameter("ACLs are marked as required for destination %s but are not supported on this platform. Set `acls: off` to resolve this error.", dd.Path)
		}

		if len(dd.Readers) > 0 {
			log.WarnContext(
				context.TODO(),
				"Readers are configured but this platform does not support filesystem ACLs. ACL management will be disabled.",
				"path", dd.Path,
			)
		}
	}

	stat, err := os.Stat(dd.Path)
	if err != nil {
		return trace.Wrap(err)
	}

	// Note: os.Getuid() returns -1 on Windows, but we've implicitly eliminated
	// that as an option with the `botfs.HasACLSupport()` check above.
	ownedByBot, err := botfs.IsOwnedBy(stat, os.Getuid())
	if trace.IsNotImplemented(err) {
		// If file owners aren't supported, ACLs certainly aren't. Just bail.
		// (Subject to change if we ever try to support Windows ACLs.)
		if dd.ACLs == botfs.ACLRequired {
			return trace.NotImplemented("unable to determine file ownership on this platform but ACLs were marked as required, cannot continue")
		}

		log.WarnContext(
			context.TODO(),
			"unable to determine file ownership on this platform, ACL verification will be skipped",
			"path", dd.Path,
			"error", err,
		)

		return nil
	} else if err != nil {
		return trace.Wrap(err)
	}

	// Check for new-style ACL configuration. These explicitly require
	// destinations to be owned by the bot user. This is somewhat arbitrarily
	// here instead of in Init() and will run before each write attempt.
	if len(dd.Readers) > 0 {
		if !ownedByBot {
			const msg = "This destination is not owned by the bot user so ACLs " +
				"will not be enforced. To silence this warning, fix destination " +
				"file ownership or remove configured `readers`"
			log.WarnContext(
				context.TODO(),
				msg,
				"path", dd.Path,
			)

			return nil
		}

		return nil
	}

	// For legacy ACLs, make sure it's worth warning about them. If the
	// destination is owned by the bot (implying the user is not trying to use
	// ACLs), just bail.
	if ownedByBot {
		return nil
	}

	return trace.Wrap(dd.verifyLegacyACLs(keys))
}

// verifyAndCorrectACL performs validation and attempts correction on new-style
// ACLs when configured.
//
//nolint:staticcheck // staticcheck doesn't like nop implementations in fs_other.go
func (dd *DestinationDirectory) verifyAndCorrectACL(ctx context.Context, subpath string) error {
	p := filepath.Join(dd.Path, subpath)

	// As a sanity check, try to ensure the resulting path is (lexically) a
	// child of this destination.
	if !strings.HasPrefix(p, filepath.Clean(dd.Path)) {
		return trace.BadParameter("path %s is not a child of destination %s", p, dd.Path)
	}

	log.DebugContext(ctx, "verifying ACL", "path", p)
	issues, err := botfs.VerifyACL(p, dd.Readers)
	if err != nil {
		return trace.Wrap(err)
	}

	// If issues exist, attempt remediation. As the test during `Init()`
	// passed, regardless of ACL mode (try, required), treat all errors here
	// as hard failures.
	if len(issues) > 0 {
		if err := botfs.ConfigureACL(p, dd.Readers); err != nil {
			return trace.Wrap(err, "unable to fix misconfigured ACL at path %s with issues %v", p, issues)
		}

		log.InfoContext(
			ctx,
			"corrected ACLs on destination file",
			"destination", dd.Path,
			"path", subpath,
			"issues", issues,
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

	// If there are any parent directories, attempt to mkdir them again in case
	// things have drifted since `Init()` was run. We don't bother with secure
	// botfs.Create() since it's a no-op for directory creation.
	if dir, _ := filepath.Split(name); dir != "" {
		if err := mkdir(filepath.Join(dd.Path, dir)); err != nil {
			return trace.Wrap(err)
		}
	}

	path := filepath.Join(dd.Path, name)

	// We can't configure permissions on a file that doesn't exist, so attempt
	// to read the file first to determine if it exists, and create it if
	// necessary.
	_, err := botfs.Read(path, dd.Symlinks)
	if trace.IsNotFound(err) {
		log.DebugContext(ctx, "file is missing, creating to apply permissions", "path", path)
		if err := botfs.Create(path, false, dd.Symlinks); err != nil {
			return trace.Wrap(err)
		}
	} else if err != nil {
		return trace.Wrap(err)
	}

	if dd.aclsEnabled {
		// Sequentially check all subdirectory ACLs.
		dirs := strings.Split(name, string(os.PathSeparator))
		for i := 1; i < len(dirs); i++ {
			// note: filepath.Join() will call `filepath.Clean()` on each result
			if err := dd.verifyAndCorrectACL(ctx, filepath.Join(dirs[:i]...)); err != nil {
				return trace.Wrap(err)
			}
		}

		if err := dd.verifyAndCorrectACL(ctx, name); err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(botfs.Write(path, data, dd.Symlinks))
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

func (dm *DestinationDirectory) MarshalYAML() (any, error) {
	type raw DestinationDirectory
	return withTypeHeader((*raw)(dm), DestinationDirectoryType)
}

func (dd *DestinationDirectory) IsPersistent() bool {
	return true
}
