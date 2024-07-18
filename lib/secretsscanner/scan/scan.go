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

package scan

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	"github.com/gravitational/teleport/api/types/accessgraph"
)

// ScannerConfig specifies parameters for the scanner.
type ScannerConfig struct {
	Dirs []string
	Log  *slog.Logger
}

// NewScanner creates a new scanner.
func NewScanner(cfg ScannerConfig) (*Scanner, error) {
	if len(cfg.Dirs) == 0 {
		return nil, trace.BadParameter("missing dirs")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Scanner{
		dirs:        cfg.Dirs,
		log:         cfg.Log,
		privateKeys: make(map[string]*accessgraphsecretsv1pb.PrivateKey),
	}, nil
}

// Scanner is a scanner that scans directories for secrets.
type Scanner struct {
	dirs []string
	log  *slog.Logger
	// privateKeys is a map of private keys found during the scan.
	// The key is the path to the private key file and the value is the private key representation.
	privateKeys map[string]*accessgraphsecretsv1pb.PrivateKey
}

// ScanPrivateKeys scans directories for SSH private keys.
func (s *Scanner) ScanPrivateKeys(ctx context.Context, deviceID string) []SSHPrivateKey {
	for _, dir := range s.dirs {
		s.findPrivateKeys(ctx, dir, deviceID)
	}

	keys := make([]SSHPrivateKey, 0, len(s.privateKeys))
	for path, key := range s.privateKeys {
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
func (s *Scanner) findPrivateKeys(ctx context.Context, root, deviceID string) {
	logger := s.log.With("dir", root)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		switch fileData, isKey, err := readFileIfSSHPrivateKey(path); {
		case err != nil:
			logger.DebugContext(ctx, "error reading file", "path", path, "error", err)
		case isKey:
			key, err := extractSSHKey(ctx, path, deviceID, fileData)
			if err != nil {
				logger.DebugContext(ctx, "error extracting private key", "path", path, "error", err)
			} else {
				s.privateKeys[path] = key
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
func readFileIfSSHPrivateKey(filePath string) ([]byte, bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	// read the first 150 bytes of the file to check if it's an OpenSSH private key.
	var buf [150]byte
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
		publicKey, mode = parsePublicKeyFromPublicPath(ctx, logger, path)
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

// parsePublicKeyFromPublicPath tries to read the public key from the public key file if the private key is password protected.
// If the public key file doesn't exist, it will return mode accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED
// identifying that the private key is password protected and the public key could not be extracted.
func parsePublicKeyFromPublicPath(ctx context.Context, logger *slog.Logger, path string) (publicKey ssh.PublicKey, mode accessgraphsecretsv1pb.PublicKeyMode) {
	mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PROTECTED

	pubPath := path + ".pub"
	logger = logger.With("public_key_file", pubPath)
	logger.DebugContext(ctx, "PrivateKey is password protected. Fallback to public key file.")

	switch pubData, err := os.ReadFile(pubPath); {
	case err != nil:
		logger.DebugContext(ctx, "Unable to read public key file.", "err", err)
		return nil, mode
	default:
		logger.DebugContext(ctx, "Trying to parse public key as authorized key data.")
		if pub, _, _, _, err := ssh.ParseAuthorizedKey(pubData); err == nil {
			publicKey = pub
			mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
			return publicKey, mode
		} else {
			logger.DebugContext(ctx, "Unable to parse ssh public key file.", "err", err)
		}

		logger.DebugContext(ctx, "Trying to parse public key directly.")
		if pub, err := ssh.ParsePublicKey(pubData); err == nil {
			publicKey = pub
			mode = accessgraphsecretsv1pb.PublicKeyMode_PUBLIC_KEY_MODE_PUB_FILE
			return publicKey, mode
		} else {
			logger.DebugContext(ctx, "Unable to parse ssh public key file.", "err", err)
		}

		return nil, mode
	}
}

func privateKeyNameGen(path, deviceID, fingerprint string) string {
	sha := sha256.New()
	sha.Write([]byte(path))
	sha.Write([]byte(deviceID))
	sha.Write([]byte(fingerprint))
	return hex.EncodeToString(sha.Sum(nil))
}
