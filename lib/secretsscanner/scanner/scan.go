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

package scanner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
)

// Config specifies parameters for the scanner.
type Config struct {
	// Dirs is a list of directories to scan.
	Dirs []string
	// SkipPaths is a list of paths to skip.
	// It supports glob patterns (e.g. "/etc/*/").
	// Please refer to the [filepath.Match] documentation for more information.
	SkipPaths []string
	// Log is the logger.
	Log *slog.Logger
}

// New creates a new scanner.
func New(cfg Config) (*Scanner, error) {
	if len(cfg.Dirs) == 0 {
		return nil, trace.BadParameter("missing dirs")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	// expand the glob patterns in the skipPaths list.
	// we expand the glob patterns here to avoid expanding them for each file during the scan.
	// only the directories matched by the glob patterns will be skipped.
	skippedPaths, err := expandSkipPaths(cfg.SkipPaths)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Scanner{
		dirs:         cfg.Dirs,
		log:          cfg.Log,
		skippedPaths: skippedPaths,
	}, nil
}

// Scanner is a scanner that scans directories for secrets.
type Scanner struct {
	dirs         []string
	log          *slog.Logger
	skippedPaths map[string]struct{}
}

// ScanPrivateKeys scans directories for SSH private keys.
func (s *Scanner) ScanPrivateKeys(ctx context.Context, deviceID string) []SSHPrivateKey {
	// privateKeys is a map of private keys found during the scan.
	// The key is the path to the private key file and the value is the private key representation.
	privateKeysMap := make(map[string]*accessgraphsecretsv1pb.PrivateKey)
	for _, dir := range s.dirs {
		s.findPrivateKeys(ctx, dir, deviceID, privateKeysMap)
	}

	keys := make([]SSHPrivateKey, 0, len(privateKeysMap))
	for path, key := range privateKeysMap {
		keys = append(keys, SSHPrivateKey{
			Path: path,
			Key:  key,
		})
	}
	return keys
}

// SSHPrivateKey represents an SSH private key found during the scan.
type SSHPrivateKey struct {
	// Path is the absolute path to the private key file.
	Path string
	// Key is the private key representation.
	Key *accessgraphsecretsv1pb.PrivateKey
}

// findPrivateKeys walks through all files in a directory and its subdirectories
// and checks if they are SSH private keys.
func (s *Scanner) findPrivateKeys(ctx context.Context, root, deviceID string, privateKeysMap map[string]*accessgraphsecretsv1pb.PrivateKey) {
	logger := s.log.With("root", root)

	err := filepath.WalkDir(root, func(path string, info fs.DirEntry, err error) error {
		// check if the context is done before processing the file.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			logger.DebugContext(ctx, "error walking directory", "path", path, "error", err)
			return fs.SkipDir
		}
		if info.IsDir() {
			if _, ok := s.skippedPaths[path]; ok {
				logger.DebugContext(ctx, "skipping directory", "path", path)
				return fs.SkipDir
			}
			return nil
		}

		if _, ok := s.skippedPaths[path]; ok {
			logger.DebugContext(ctx, "skipping file", "path", path)
			return nil
		}

		switch fileData, isKey, err := s.readFileIfSSHPrivateKey(ctx, path); {
		case err != nil:
			logger.DebugContext(ctx, "error reading file", "path", path, "error", err)
		case isKey:
			key, err := extractSSHKey(ctx, path, deviceID, fileData)
			if err != nil {
				logger.DebugContext(ctx, "error extracting private key", "path", path, "error", err)
			} else {
				privateKeysMap[path] = key
			}
		}
		return nil
	})

	if err != nil {
		logger.WarnContext(ctx, "error walking directory", "root", root, "error", err)
	}
}

var (
	supportedPrivateKeyHeaders = [][]byte{
		[]byte("RSA PRIVATE KEY"),
		[]byte("PRIVATE KEY"),
		[]byte("EC PRIVATE KEY"),
		[]byte("DSA PRIVATE KEY"),
		[]byte("OPENSSH PRIVATE KEY"),
	}
)

// readFileIfSSHPrivateKey checks if a file is an OpenSSH private key
func (s *Scanner) readFileIfSSHPrivateKey(ctx context.Context, filePath string) ([]byte, bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		if err = file.Close(); err != nil {
			s.log.DebugContext(ctx, "failed to close file", "path", filePath, "error", err)
		}
	}()

	// read the first bytes of the file to check if it's an OpenSSH private key.
	// 40 bytes is the maximum length of the header of an OpenSSH private key.
	var buf [40]byte
	n, err := file.Read(buf[:])
	if errors.Is(err, io.EOF) || n < len(buf) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, trace.Wrap(err, "failed to read file")
	}

	isPrivateKey := false
	for _, header := range supportedPrivateKeyHeaders {
		if bytes.Contains(buf[:], header) {
			isPrivateKey = true
			break
		}
	}
	if !isPrivateKey {
		return nil, false, nil
	}

	// read the entire file
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, false, trace.Wrap(err, "failed to read file")
	}
	return append(buf[:], data...), true, nil
}

func extractSSHKey(ctx context.Context, path, deviceID string, fileData []byte) (*accessgraphsecretsv1pb.PrivateKey, error) {
	logger := slog.Default().With("private_key_file", path, "device_id", deviceID)

	var publicKey ssh.PublicKey
	var mode accessgraphsecretsv1pb.PublicKeyMode
	var pme *ssh.PassphraseMissingError
	switch pk, err := ssh.ParsePrivateKey(fileData); {
	case errors.As(err, &pme):
		if pme.PublicKey != nil {
			// If the key is a OpenSSH private key whose public key is embedded in the header, it will return the public key. This is
			// a special case for OpenSSH private keys that have the public key embedded in the header, for more information see
			// OpenSSH's ssh key format: https://github.com/openssh/openssh-portable/blob/master/PROTOCOL.key
			publicKey = pme.PublicKey
			mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED
			break
		}
		const pubKeyFileSuffix = ".pub"
		publicKey, mode = tryParsingPublicKeyFromPublicFilePath(ctx, logger, path+pubKeyFileSuffix)
	case err != nil:
		return nil, trace.Wrap(err)
	default:
		publicKey = pk.PublicKey()
		mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_DERIVED
	}
	var fingerprint string
	if publicKey != nil {
		fingerprint = ssh.FingerprintSHA256(publicKey)
	}

	key, err := accessgraph.NewPrivateKeyWithName(
		privateKeyNameGen(path, deviceID, fingerprint),
		&accessgraphsecretsv1pb.PrivateKeySpec{
			PublicKeyFingerprint: fingerprint,
			DeviceId:             deviceID,
			PublicKeyMode:        mode,
		},
	)
	return key, trace.Wrap(err)
}

// tryParsingPublicKeyFromPublicFilePath tries to read the public key from the public key file if the private key is password protected.
// If the public key file doesn't exist, it will return mode accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED
// identifying that the private key is password protected and the public key could not be extracted.
func tryParsingPublicKeyFromPublicFilePath(ctx context.Context, logger *slog.Logger, pubPath string) (ssh.PublicKey, accessgraphsecretsv1pb.PublicKeyMode) {
	logger = logger.With("public_key_file", pubPath)
	logger.DebugContext(ctx, "PrivateKey is password protected. Fallback to public key file.")

	pubData, err := os.ReadFile(pubPath)
	if err != nil {
		logger.DebugContext(ctx, "Unable to read public key file.", "err", err)
		return nil, accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED
	}

	logger.DebugContext(ctx, "Trying to parse public key as authorized key data.")
	pub, _, _, _, err := ssh.ParseAuthorizedKey(pubData)
	if err == nil {
		return pub, accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
	}
	logger.DebugContext(ctx, "Unable to parse ssh public key file.", "err", err)

	logger.DebugContext(ctx, "Trying to parse public key directly.")

	pub, err = ssh.ParsePublicKey(pubData)
	if err == nil {
		return pub, accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
	}

	logger.DebugContext(ctx, "Unable to parse ssh public key file.", "err", err)

	return nil, accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED

}

func privateKeyNameGen(path, deviceID, fingerprint string) string {
	sha := sha256.New()
	sha.Write([]byte(path))
	sha.Write([]byte(deviceID))
	sha.Write([]byte(fingerprint))
	return hex.EncodeToString(sha.Sum(nil))
}

// expandSkipPaths expands the glob patterns in the skipPaths list and returns a set of the
// paths matched by the glob patterns to be skipped.
func expandSkipPaths(skipPaths []string) (map[string]struct{}, error) {
	set := make(map[string]struct{})
	for _, glob := range skipPaths {
		matches, err := filepath.Glob(glob)
		if err != nil {
			return nil, trace.Wrap(err, "glob pattern %q is invalid", glob)
		}
		for _, match := range matches {
			set[match] = struct{}{}
		}
	}
	return set, nil
}
