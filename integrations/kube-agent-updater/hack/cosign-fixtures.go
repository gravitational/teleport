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

// cosign-fixtures is a tool to generate valid and invalid cosign signed artifacts
// This is used to test the Cosign validator implementation.
// This could be dynamically executed on every test but cryptographic operations
// are costly and this would make things a bit less debuggable.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"maps"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/gravitational/trace"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci"
	staticsign "github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

const (
	// imagePath to put in the signature. The registry and repository are not
	// validated by cosign.SimpleClaimVerifier so we can but an arbitrary
	// value here
	imagePath = "localhost:5000/repo/image"
	// wrongHash is an arbitrary Hash used to sign payloads pointing to the wrong digest
	wrongHash = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"
)

var (
	dummyConfig = static.NewLayer([]byte("dummy config"), types.OCIUncompressedLayer)
)

func main() {
	manifests := map[string]v1.Manifest{}
	fixtures := map[string]v1.Hash{}

	layers := []v1.Layer{dummyConfig}

	// We generate new keys for each generation because if we store test private
	// keys in the code repo, we'll trigger all code scanners. It reduces
	// reproducibility, but it's still less painful than dealing with false
	// positive reports.
	validKey, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return []byte{}, nil })
	if err != nil {
		log.Fatal(err)
	}
	invalidKeyA, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return []byte{}, nil })
	if err != nil {
		log.Fatal(err)
	}
	invalidKeyB, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return []byte{}, nil })
	if err != nil {
		log.Fatal(err)
	}

	// Generate manifests and layers for every test case
	l, m, h, err := generateSignedManifest("signed manifest", signDigestedRef, validKey)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["signedManifest"] = h

	l, m, h, err = generateSignedManifest("unsigned manifest", signDigestedRef)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["unsignedManifest"] = h

	l, m, h, err = generateSignedManifest("untrusted signed manifest", signDigestedRef, invalidKeyA)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["untrustedSignedManifest"] = h

	l, m, h, err = generateSignedManifest("double signed manifest", signDigestedRef, invalidKeyA, validKey)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["doubleSignedManifest"] = h

	l, m, h, err = generateSignedManifest("untrusted double signed manifest", signDigestedRef, invalidKeyA, invalidKeyB)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["untrustedDoubleSignedManifest"] = h

	l, m, h, err = generateSignedManifest("wrongly signed manifest", wronglySignDigestedRef, validKey)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["wronglySignedManifest"] = h

	l, m, h, err = generateSignedManifest("untrusted wrongly signed manifest", wronglySignDigestedRef, invalidKeyA)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["untrustedWronglySignedManifest"] = h

	l, m, h, err = generateSignedIndex("signed index", signDigestedRef, validKey)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["signedIndex"] = h

	l, m, h, err = generateSignedIndex("unsigned index", signDigestedRef)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["unsignedIndex"] = h

	l, m, h, err = generateSignedIndex("untrusted signed index", signDigestedRef, invalidKeyA)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["untrustedSignedIndex"] = h

	l, m, h, err = generateSignedIndex("double signed index", signDigestedRef, invalidKeyA, validKey)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["doubleSignedIndex"] = h

	l, m, h, err = generateSignedIndex("untrusted double signed index", signDigestedRef, invalidKeyA, invalidKeyB)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["untrustedDoubleSignedIndex"] = h

	l, m, h, err = generateSignedIndex("wrongly signed index", wronglySignDigestedRef, validKey)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["wronglySignedIndex"] = h

	l, m, h, err = generateSignedIndex("untrusted wrongly signed index", wronglySignDigestedRef, invalidKeyA)
	if err != nil {
		log.Fatal(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	fixtures["untrustdedWronglySignedIndex"] = h

	// Export and exit
	result, err := renderFixtures(validKey, layers, manifests, fixtures)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}

type digestedRefSigner func(name.Digest, *cosign.KeysBytes) (oci.Signature, error)

// signDigestedRef receives a name.Digest reference and sign it using cosign.KeysBytes.
// The result is am oci.Signature, which extends v1.Layer by adding the Annotations()
// and Signature() method.
func signDigestedRef(manifestRef name.Digest, key *cosign.KeysBytes) (oci.Signature, error) {
	sigPayload := payload.Cosign{
		Image: manifestRef,
	}

	return signPayload(sigPayload, key)
}

// wronglySignDigestedRef receives a name.Digest reference and sign it using cosign.KeysBytes.
// The result is a valid oci.Signature want a payload referring to a different
// manifest than the one provided.
func wronglySignDigestedRef(manifestRef name.Digest, key *cosign.KeysBytes) (oci.Signature, error) {
	manifestRefNoHash := strings.Split(manifestRef.String(), "@")[0]
	wrongManifest, err := name.NewDigest(manifestRefNoHash + "@sha256:" + wrongHash)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sigPayload := payload.Cosign{
		Image: wrongManifest,
	}

	return signPayload(sigPayload, key)
}

// signPayload signs a payload with a key.
func signPayload(sigPayload payload.Cosign, key *cosign.KeysBytes) (oci.Signature, error) {
	sigPayloadBytes, err := json.Marshal(sigPayload)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sv, err := cosign.LoadPrivateKey(key.PrivateBytes, []byte{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sig, err := sv.SignMessage(bytes.NewReader(sigPayloadBytes))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b64sig := base64.StdEncoding.EncodeToString(sig)
	ociSig, err := staticsign.NewSignature(sigPayloadBytes, b64sig)
	return ociSig, trace.Wrap(err)
}

// descriptorFromLayer take a layer and returns a v1.Descriptor pointing to it.
// This is used when building v1.Manifests to reference a known layer or signature.
func descriptorFromLayer(layer v1.Layer) (v1.Descriptor, error) {
	layerType, err := layer.MediaType()
	if err != nil {
		return v1.Descriptor{}, trace.Wrap(err)
	}
	layerSize, err := layer.Size()
	if err != nil {
		return v1.Descriptor{}, trace.Wrap(err)
	}
	layerDigest, err := layer.Digest()
	if err != nil {
		return v1.Descriptor{}, trace.Wrap(err)
	}

	var layerAnnotations map[string]string
	if sig, ok := layer.(oci.Signature); ok {
		layerAnnotations, err = sig.Annotations()
		if err != nil {
			return v1.Descriptor{}, trace.Wrap(err)
		}
	}

	return v1.Descriptor{
		MediaType:   layerType,
		Size:        layerSize,
		Digest:      layerDigest,
		Annotations: layerAnnotations,
	}, nil
}

// descriptorFromManifest take a v1.Manifest and returns a v1.Descriptor pointing to it.
// This is used when building v1.Index to reference a known manifest.
func descriptorFromManifest(manifest v1.Manifest, platform *v1.Platform) (v1.Descriptor, error) {
	_, size, hash, err := contentSizeAndHash(manifest)
	if err != nil {
		return v1.Descriptor{}, trace.Wrap(err)
	}
	return v1.Descriptor{
		MediaType: types.DockerManifestSchema2,
		Size:      size,
		Digest:    hash,
		Platform:  platform,
	}, err

}

// makeSignature signs a given name.Digest using a digestedRefSigner with each Key provided.
// It returns signature payload layers and a signature Manifest referencing
// all signatures layers.
func makeSignature(ref name.Digest, signer digestedRefSigner, keys ...*cosign.KeysBytes) ([]v1.Layer, v1.Manifest, error) {
	// Generating signatures
	configDesc, err := descriptorFromLayer(dummyConfig)
	if err != nil {
		return nil, v1.Manifest{}, trace.Wrap(err)
	}

	var sigDescriptors []v1.Descriptor
	var layers []v1.Layer
	for _, key := range keys {
		// Technically all signatures will have the same payload, hashes will conflict
		// and only the last layer will be available. That's not an issue as long as
		// there are multiple references with different annotations to the unique layer
		// in the signature Manifest.
		ociSig, err := signer(ref, key)
		if err != nil {
			return nil, v1.Manifest{}, trace.Wrap(err)
		}
		ociSigDesc, err := descriptorFromLayer(ociSig)
		if err != nil {
			return nil, v1.Manifest{}, trace.Wrap(err)
		}
		sigDescriptors = append(sigDescriptors, ociSigDesc)
		layers = append(layers, ociSig)
	}

	sigManifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config:        configDesc,
		Layers:        sigDescriptors,
	}

	return layers, sigManifest, nil
}

// contentSizeAndHash serializes a manifest or a manifest index and computes its v1.Hash.
// This is used to know under which digest the manifest will be reachable, for
// example when building a signature referring to the manifest or when building
// a manifest index.
func contentSizeAndHash(obj any) ([]byte, int64, v1.Hash, error) {
	manifestBytes, err := json.Marshal(obj)
	if err != nil {
		return manifestBytes, 0, v1.Hash{}, trace.Wrap(err)
	}
	manifestDigest, size, err := v1.SHA256(bytes.NewReader(manifestBytes))
	return manifestBytes, size, manifestDigest, trace.Wrap(err)
}

// Triangulate takes a v1.Hash and returns the tag where the Cosign signature
// of the hashed object is expected to live. This is equivalent to the
// `cosign triangulate` command
func Triangulate(h v1.Hash) string {
	return fmt.Sprintf("%s-%s.sig", h.Algorithm, h.Hex)
}

const fixtureTemplate = `/*
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

package img

// Content generated by "hack/cosign-fixtures.go". Do not edit manually.

var publicKey = []byte({{ .PublicKey | quote }})

var blobs = map[string]string{
{{- range $hash, $content := .Blobs }}
	{{ $hash | quote }}: {{ $content | quote }},
{{- end }}
}

var manifests = map[string]string{
{{- range $hash, $content := .Manifests }}
	{{ $hash | quote }}: {{ $content | quote }},
{{- end }}
}

const (
{{- range $ref, $hash := .Refs }}
	{{ $ref }} = {{ $hash | quote }}
{{- end }}
)
`

type fixtureTemplateValues struct {
	PrivateKey string
	PublicKey  string
	Blobs      map[string]string
	Manifests  map[string]string
	Refs       map[string]string
}

func renderFixtures(keys *cosign.KeysBytes, layers []v1.Layer, manifests map[string]v1.Manifest, refs map[string]v1.Hash) (string, error) {
	values := fixtureTemplateValues{
		PrivateKey: string(keys.PrivateBytes),
		PublicKey:  string(keys.PublicBytes),
		Blobs:      map[string]string{},
		Manifests:  map[string]string{},
		Refs:       map[string]string{},
	}
	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return "", trace.Wrap(err)
		}
		contentReader, err := layer.Uncompressed()
		if err != nil {
			return "", trace.Wrap(err)
		}
		content, err := io.ReadAll(contentReader)
		if err != nil {
			return "", trace.Wrap(err)
		}
		err = contentReader.Close()
		if err != nil {
			return "", trace.Wrap(err)
		}
		values.Blobs[digest.String()] = string(content)
	}

	for ref, manifest := range manifests {
		content, _, _, err := contentSizeAndHash(manifest)
		if err != nil {
			return "", trace.Wrap(err)
		}
		values.Manifests[ref] = string(content)
	}

	for ref, hash := range refs {
		values.Refs[ref] = hash.String()
	}

	var tpl bytes.Buffer
	t := template.Must(template.New("fixtures").Funcs(sprig.FuncMap()).Parse(fixtureTemplate))
	err := t.Execute(&tpl, values)
	return tpl.String(), trace.Wrap(err)
}

// generates the fixtures for the signedManifest tests
// The function can be used to generate multiple manifests with different keys
// To ensure the hashes are different and fixtures don't conflict together, we
// put the scenario name in the layers.
func generateSignedManifest(scenario string, signer digestedRefSigner, keys ...*cosign.KeysBytes) (layers []v1.Layer, manifests map[string]v1.Manifest, hash v1.Hash, err error) {
	manifests = map[string]v1.Manifest{}
	// Generating test layers
	configDesc, err := descriptorFromLayer(dummyConfig)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	layer := static.NewLayer([]byte(scenario+" layer"), types.OCIUncompressedLayer)
	layerDesc, err := descriptorFromLayer(layer)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}

	layers = append(layers, layer)
	manifest := v1.Manifest{
		SchemaVersion: 2,
		MediaType:     types.OCIManifestSchema1,
		Config:        configDesc,
		Layers:        []v1.Descriptor{layerDesc},
	}

	// Generating a manifest referencing the test layers
	_, _, manifestDigest, err := contentSizeAndHash(manifest)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	hash = manifestDigest
	manifests[manifestDigest.String()] = manifest

	manifestRef, err := name.NewDigest(imagePath + "@" + manifestDigest.String())

	// Don't sign when no keys are provided
	if len(keys) == 0 {
		return
	}

	// Signing the manifest
	sigLayers, sigManifest, err := makeSignature(manifestRef, signer, keys...)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	manifests[Triangulate(manifestDigest)] = sigManifest
	layers = append(layers, sigLayers...)
	return
}

// generates the fixtures for the signedIndex test
// The function generates two signed manifests and creates an index referencing
// both manifests under different Platforms. Both individual manifests and the
// index are signed using the provided keys and signer function.
func generateSignedIndex(scenario string, signer digestedRefSigner, keys ...*cosign.KeysBytes) (layers []v1.Layer, manifests map[string]v1.Manifest, hash v1.Hash, err error) {
	manifests = map[string]v1.Manifest{}

	// Generating two manifests A and B
	l, m, h, err := generateSignedManifest(scenario+" manifest A", signDigestedRef)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	manifestA := m[h.String()]
	manifestADesc, err := descriptorFromManifest(manifestA, &v1.Platform{
		Architecture: "arm64",
		OS:           "linux",
	})
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}

	l, m, h, err = generateSignedManifest(scenario+" manifest B", signDigestedRef)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	layers = append(layers, l...)
	maps.Copy(manifests, m)
	manifestB := m[h.String()]
	manifestBDesc, err := descriptorFromManifest(manifestB, &v1.Platform{
		Architecture: "amd64",
		OS:           "linux",
	})
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}

	// Referencing both manifests in an index
	index := v1.IndexManifest{
		SchemaVersion: 2,
		MediaType:     types.DockerManifestList,
		Manifests:     []v1.Descriptor{manifestADesc, manifestBDesc},
	}

	_, _, indexDigest, err := contentSizeAndHash(index)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	hash = indexDigest
	indexRef, err := name.NewDigest(imagePath + "@" + indexDigest.String())

	// Don't sign when no keys are provided
	if len(keys) == 0 {
		return
	}

	// Signing the index
	sigLayers, sigManifest, err := makeSignature(indexRef, signer, keys...)
	if err != nil {
		return nil, nil, v1.Hash{}, trace.Wrap(err)
	}
	manifests[Triangulate(indexDigest)] = sigManifest
	layers = append(layers, sigLayers...)
	return
}
