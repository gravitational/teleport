/*
Copyright 2025 Gravitational, Inc.

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

package client

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"strconv"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
)

var (
	errLocked   = trace.AccessDenied("extendedAgent: locked")
	errNotFound = trace.NotFound("extendedAgent: key not found")
)

// extendedAgent is a wrapper for an agent with extensions.
type extendedAgent struct {
	// agent is the underlying agent.
	agent agent.ExtendedAgent
	// extensionHandlers are used to handle agent extension requests. These
	// handlers are called under mu.Lock for safe access to protected fields.
	extensionHandlers map[string]extensionHandler

	mu sync.Mutex
	// locked locks the extended agent.
	locked bool
	// cryptoSigners are the corresponding crypto.Signers for added agent keys.
	cryptoSigners map[string]crypto.Signer
}

// NewExtendedAgent returns an extended agent wrapper for the given agent.
func NewExtendedAgent(agent agent.ExtendedAgent, extensions ...AgentExtension) (agent.ExtendedAgent, error) {
	extendedAgent := &extendedAgent{
		agent:             agent,
		cryptoSigners:     make(map[string]crypto.Signer),
		extensionHandlers: make(map[string]extensionHandler),
	}

	for _, e := range extensions {
		extendedAgent.extensionHandlers[e.name] = e.handler
	}

	return extendedAgent, nil
}

// AgentExtensionOpt holds details for a specific ssh agent extension.
type AgentExtension struct {
	name    string
	handler extensionHandler
}

// extensionHandler handles an agent extension request.
type extensionHandler func(a *extendedAgent, contents []byte) ([]byte, error)

// WithSignExtension returns the AgentExtension for sign@goteleport.com.
// This enabled forwarded keys to be used as a crypto.Signer in addition to the usual
// ssh.Signer, enabling non ssh cryptographic operations, such as TLS handshakes.
func WithSignExtension() AgentExtension {
	return AgentExtension{
		name:    signAgentExtension,
		handler: signExtensionHandler(),
	}
}

// WithSignExtension returns the AgentExtension for key@goteleport.com.
// This extension can be used to retrieve a client's profile and certs from the client store.
func WithKeyExtension(s *Store) AgentExtension {
	return AgentExtension{
		name:    keyAgentExtension,
		handler: keyExtensionHandler(s),
	}
}

// RemoveAll removes all identities.
func (a *extendedAgent) RemoveAll() error {
	if err := a.agent.RemoveAll(); err != nil {
		return trace.Wrap(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.cryptoSigners = make(map[string]crypto.Signer)
	return nil
}

// Remove removes all identities with the given public key.
func (a *extendedAgent) Remove(key ssh.PublicKey) error {
	if err := a.agent.Remove(key); err != nil {
		return trace.Wrap(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.cryptoSigners, string(key.Marshal()))
	return nil
}

// Lock locks the agent. Sign, Remove, and Extension will fail, and List will return an empty list.
func (a *extendedAgent) Lock(passphrase []byte) error {
	if err := a.agent.Lock(passphrase); err != nil {
		return trace.Wrap(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.locked = true
	return nil
}

// Unlock undoes the effect of Lock
func (a *extendedAgent) Unlock(passphrase []byte) error {
	if err := a.agent.Unlock(passphrase); err != nil {
		return trace.Wrap(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.locked = false
	return nil
}

// List returns the identities known to the agent.
func (a *extendedAgent) List() ([]*agent.Key, error) {
	return a.agent.List()
}

// Insert adds a private key to the agent. If a certificate
// is given, that certificate is added as public key. Note that
// any constraints given are ignored.
func (a *extendedAgent) Add(key agent.AddedKey) error {
	cryptoSigner, ok := key.PrivateKey.(crypto.Signer)
	if !ok {
		return trace.BadParameter("invalid agent key: signer of type %T does not implement crypto.Signer", cryptoSigner)
	}

	var sshPub ssh.PublicKey = key.Certificate
	if key.Certificate == nil {
		var err error
		if sshPub, err = ssh.NewPublicKey(cryptoSigner.Public()); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := a.agent.Add(key); err != nil {
		return trace.Wrap(err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.cryptoSigners[string(sshPub.Marshal())] = cryptoSigner
	return nil
}

// Sign returns a signature for the data.
func (a *extendedAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.agent.Sign(key, data)
}

// SignWithFlags signs like Sign, but allows for additional flags to be sent/received.
func (a *extendedAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	return a.agent.SignWithFlags(key, data, flags)
}

// Signers returns signers for all the known keys.
func (a *extendedAgent) Signers() ([]ssh.Signer, error) {
	return a.agent.Signers()
}

// cryptoSignUnderLock returns a signature for the data using the sign@goteleport.com extension.
// This method should be called under a.mu.Lock.
func (a *extendedAgent) cryptoSignUnderLock(key ssh.PublicKey, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	cryptoSigner, ok := a.cryptoSigners[string(key.Marshal())]
	if !ok {
		return nil, errNotFound
	}

	return cryptoSigner.Sign(rand.Reader, digest, opts)
}

const (
	// the query extension is a common extension used to query what extensions are supported.
	queryAgentExtension = "query"

	// the key extension can be used to retrieve teleport client certificates from the
	// remote agent. These certificates can be used in concert with the sign extension
	// to produce a remote client key.
	keyAgentExtension = "key@goteleport.com"

	// the sign extension can be used to issue a standard signature request to an agent
	// key, rather than an ssh style signature.
	signAgentExtension = "sign@goteleport.com"

	// Used to indicate that the salt length will be set during signing to the largest
	// value possible. This salt length can then be auto-detected during verification.
	saltLengthAuto = "auto"
)

// The extendedAgent may support extensions provided during creation.
func (a *extendedAgent) Extension(extensionType string, contents []byte) ([]byte, error) {
	if extensionType == queryAgentExtension {
		return a.queryExtension()
	}

	if handler, ok := a.extensionHandlers[extensionType]; ok {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.locked {
			return nil, errLocked
		}
		return handler(a, contents)
	}
	return nil, agent.ErrExtensionUnsupported
}

// QueryExtensionResponse is a response object for the query extension.
type QueryExtensionResponse struct {
	// ExtensionsNames is a list of supported extensions.
	ExtensionNames []string
}

// queryExtension returns a list of supported extensions.
func (a *extendedAgent) queryExtension() ([]byte, error) {
	extensionNames := make([]string, 0, len(a.extensionHandlers))
	for extensionName := range a.extensionHandlers {
		extensionNames = append(extensionNames, extensionName)
	}

	resp := QueryExtensionResponse{
		ExtensionNames: extensionNames,
	}

	return ssh.Marshal(resp), nil
}

// callQueryExtension calls the query extension to find a map of supported extensions.
// If the extension is unsupported, an ErrExtensionUnsupported error will be returned.
func callQueryExtension(a agent.ExtendedAgent) (map[string]bool, error) {
	resp, err := a.Extension(queryAgentExtension, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var queryResponse QueryExtensionResponse
	if err := ssh.Unmarshal(resp, &queryResponse); err != nil {
		return nil, trace.Wrap(err)
	}

	supportedExtensions := make(map[string]bool)
	for _, extensionName := range queryResponse.ExtensionNames {
		supportedExtensions[extensionName] = true
	}

	return supportedExtensions, nil
}

// SignExtensionRequest is a request object for the sign@goteleport.com extension.
type SignExtensionRequest struct {
	// KeyBlob is an ssh public key in ssh wire protocol according to RFC 4253, section 6.6.
	KeyBlob []byte
	// Digest is a hashed message to sign.
	Digest []byte
	// HashName is the name of the hash used to generate the digest.
	HashName string
	// SaltLength controls the length of the salt to use in PSS signature if set.
	SaltLength string
}

// signExtensionHandler returns an extensionHandler for the sign@goteleport.com extension.
func signExtensionHandler() extensionHandler {
	return func(a *extendedAgent, contents []byte) ([]byte, error) {
		var req SignExtensionRequest
		if err := ssh.Unmarshal(contents, &req); err != nil {
			return nil, trace.Wrap(err)
		}

		sshPub, err := ssh.ParsePublicKey(req.KeyBlob)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		hash := cryptoHashFromHashName(req.HashName)
		var signerOpts crypto.SignerOpts = hash
		if req.SaltLength != "" {
			pssOpts := &rsa.PSSOptions{Hash: hash}
			if req.SaltLength == saltLengthAuto {
				pssOpts.SaltLength = rsa.PSSSaltLengthAuto
			} else {
				pssOpts.SaltLength, err = strconv.Atoi(req.SaltLength)
				if err != nil {
					return nil, trace.Wrap(err)
				}
			}
			signerOpts = pssOpts
		}

		signature, err := a.cryptoSignUnderLock(sshPub, req.Digest, signerOpts)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return ssh.Marshal(ssh.Signature{
			Format: sshPub.Type(),
			Blob:   signature,
		}), nil
	}
}

// callSignExtension calls the sign@goteleport.com extension to sign the given
// digest and return the resulting signature.
func callSignExtension(agent agent.ExtendedAgent, pub ssh.PublicKey, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	if opts == nil {
		opts = crypto.Hash(0)
	}
	req := SignExtensionRequest{
		KeyBlob:  pub.Marshal(),
		Digest:   digest,
		HashName: opts.HashFunc().String(),
	}
	if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
		switch pssOpts.SaltLength {
		case rsa.PSSSaltLengthEqualsHash:
			req.SaltLength = strconv.Itoa(opts.HashFunc().Size())
		case rsa.PSSSaltLengthAuto:
			req.SaltLength = saltLengthAuto
		default:
			req.SaltLength = strconv.Itoa(pssOpts.SaltLength)
		}
	}
	respBlob, err := agent.Extension(signAgentExtension, ssh.Marshal(req))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resp ssh.Signature
	if err := ssh.Unmarshal(respBlob, &resp); err != nil {
		return nil, trace.Wrap(err)
	}
	return resp.Blob, nil
}

// KeyExtensionResponse is a response object for the key@goteleport.com extension.
type KeyExtensionResponse struct {
	// ProfileBlob is a json encoded profile.Profile.
	ProfileBlob []byte
	// KeyBlob is a json encoded ForwardedKey.
	KeyBlob []byte
}

// ForwardedKey is a teleport client key.
type ForwardedKey struct {
	KeyRingIndex
	// SSHCertificate is a user's ssh certificate.
	SSHCertificate []byte
	// SSHCertificate is a user's tls certificate.
	TLSCertificate []byte
	// TrustedCerts is a list of trusted CAs with associated
	// TLS certificates and SSH authorized keys.
	TrustedCerts []authclient.TrustedCerts
}

// keyExtensionHandler returns an extensionHandler for the key@goteleport.com extension.
func keyExtensionHandler(s *Store) extensionHandler {
	return func(a *extendedAgent, contents []byte) ([]byte, error) {
		profileName, err := s.CurrentProfile()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		profile, err := s.GetProfile(profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		idx := KeyRingIndex{
			ProxyHost:   profileName,
			ClusterName: profile.SiteName,
			Username:    profile.Username,
		}

		key, err := s.GetKeyRing(idx, WithSSHCerts{})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// Ensure that the key is available as a crypto.Signer in the agent.
		sshpub, err := sshutils.ParseCertificate(key.Cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if _, ok := a.cryptoSigners[string(sshpub.Marshal())]; !ok {
			return nil, trace.NotFound("key not found")
		}

		profileBlob, err := json.Marshal(profile)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		forwardedKey := ForwardedKey{
			KeyRingIndex:   key.KeyRingIndex,
			SSHCertificate: key.Cert,
			TLSCertificate: key.TLSCert,
			TrustedCerts:   key.TrustedCerts,
		}

		forwardedKeyBlob, err := json.Marshal(forwardedKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return ssh.Marshal(KeyExtensionResponse{
			ProfileBlob: profileBlob,
			KeyBlob:     forwardedKeyBlob,
		}), nil
	}
}

// callKeyExtension calls the key@goteleport.com extension and returns the
// profile and forwarded key from the response.
func callKeyExtension(agent agent.ExtendedAgent) (*profile.Profile, *ForwardedKey, error) {
	respBlob, err := agent.Extension(keyAgentExtension, nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var resp KeyExtensionResponse
	if err := ssh.Unmarshal(respBlob, &resp); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var profile profile.Profile
	if err := json.Unmarshal(resp.ProfileBlob, &profile); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var forwardedKey ForwardedKey
	if err := json.Unmarshal(resp.KeyBlob, &forwardedKey); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &profile, &forwardedKey, nil
}

// Returns the crypto.Hash for the given hash name, matching the
// value returned by the hash's String method. Unknown hashes will
// return the zero hash, which will skip pre-hashing. This is used
// in some signing algorithms.
func cryptoHashFromHashName(name string) crypto.Hash {
	switch name {
	case "MD4":
		return crypto.MD4
	case "MD5":
		return crypto.MD5
	case "SHA-1":
		return crypto.SHA1
	case "SHA-224":
		return crypto.SHA224
	case "SHA-256":
		return crypto.SHA256
	case "SHA-384":
		return crypto.SHA384
	case "SHA-512":
		return crypto.SHA512
	case "MD5+SHA1":
		return crypto.MD5SHA1
	case "RIPEMD-160":
		return crypto.RIPEMD160
	case "SHA3-224":
		return crypto.SHA3_224
	case "SHA3-256":
		return crypto.SHA3_256
	case "SHA3-384":
		return crypto.SHA3_384
	case "SHA3-512":
		return crypto.SHA3_512
	case "SHA-512/224":
		return crypto.SHA512_224
	case "SHA-512/256":
		return crypto.SHA512_256
	case "BLAKE2s-256":
		return crypto.BLAKE2s_256
	case "BLAKE2b-256":
		return crypto.BLAKE2b_256
	case "BLAKE2b-384":
		return crypto.BLAKE2b_384
	case "BLAKE2b-512":
		return crypto.BLAKE2b_512
	default:
		return crypto.Hash(0)
	}
}
