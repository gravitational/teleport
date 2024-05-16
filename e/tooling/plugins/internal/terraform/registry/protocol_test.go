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
package registry

import (
	"encoding/json"
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/stretchr/testify/require"
)

const validIndex = `
{
	"versions": [
	  {
		"version": "2.0.0",
		"protocols": ["4.0", "5.1"],
		"platforms": [
		  {"os": "darwin", "arch": "amd64"},
		  {"os": "linux", "arch": "amd64"},
		  {"os": "linux", "arch": "arm"},
		  {"os": "windows", "arch": "amd64"}
		]
	  },
	  {
		"version": "2.0.1",
		"protocols": ["5.2"],
		"platforms": [
		  {"os": "darwin", "arch": "amd64"},
		  {"os": "linux", "arch": "amd64"},
		  {"os": "linux", "arch": "arm"},
		  {"os": "windows", "arch": "amd64"}
		]
	  }
	]
} 
`

func TestIndexJson(t *testing.T) {
	uut := Versions{
		Versions: []Version{
			{
				Version:   semver.Version{Major: 2, Minor: 0, Patch: 0},
				Protocols: []string{"4.0", "5.1"},
				Platforms: []Platform{
					{OS: "darwin", Arch: "amd64"},
					{OS: "linux", Arch: "amd64"},
					{OS: "linux", Arch: "arm"},
					{OS: "windows", Arch: "amd64"},
				},
			},
			{
				Version:   semver.Version{Major: 2, Minor: 0, Patch: 1},
				Protocols: []string{"5.2"},
				Platforms: []Platform{
					{OS: "darwin", Arch: "amd64"},
					{OS: "linux", Arch: "amd64"},
					{OS: "linux", Arch: "arm"},
					{OS: "windows", Arch: "amd64"},
				},
			},
		},
	}

	t.Run("Formatting", func(t *testing.T) {
		formattedUut, err := json.Marshal(&uut)
		require.NoError(t, err)

		// To avoid having to deal with trivial formatting & whitespace
		// differences we will parse the generated JSON into a generic
		// nested key-value map, and compare it against a similar map
		// generated from the expected JSON.

		var expected map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(validIndex), &expected))

		var actual map[string]interface{}
		require.NoError(t, json.Unmarshal(formattedUut, &actual))

		require.Equal(t, expected, actual)
	})

	t.Run("Parsing", func(t *testing.T) {
		var parsedIndex Versions
		require.NoError(t, json.Unmarshal([]byte(validIndex), &parsedIndex))
		require.Equal(t, uut, parsedIndex)
	})
}

const validDownloadText = `
{
	"protocols": ["4.0", "5.1"],
	"os": "linux",
	"arch": "amd64",
	"filename": "terraform-provider-random_2.0.0_linux_amd64.zip",
	"download_url": "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_linux_amd64.zip",
	"shasums_url": "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS",
	"shasums_signature_url": "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS.sig",
	"shasum": "5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a",
	"signing_keys": {
	  "gpg_public_keys": [
		{
		  "key_id": "51852D87348FFC4C",
		  "ascii_armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFMORM0BCADBRyKO1MhCirazOSVwcfTr1xUxjPvfxD3hjUwHtjsOy/bT6p9f\nW2mRPfwnq2JB5As+paL3UGDsSRDnK9KAxQb0NNF4+eVhr/EJ18s3wwXXDMjpIifq\nfIm2WyH3G+aRLTLPIpscUNKDyxFOUbsmgXAmJ46Re1fn8uKxKRHbfa39aeuEYWFA\n3drdL1WoUngvED7f+RnKBK2G6ZEpO+LDovQk19xGjiMTtPJrjMjZJ3QXqPvx5wca\nKSZLr4lMTuoTI/ZXyZy5bD4tShiZz6KcyX27cD70q2iRcEZ0poLKHyEIDAi3TM5k\nSwbbWBFd5RNPOR0qzrb/0p9ksKK48IIfH2FvABEBAAG0K0hhc2hpQ29ycCBTZWN1\ncml0eSA8c2VjdXJpdHlAaGFzaGljb3JwLmNvbT6JATgEEwECACIFAlMORM0CGwMG\nCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEFGFLYc0j/xMyWIIAIPhcVqiQ59n\nJc07gjUX0SWBJAxEG1lKxfzS4Xp+57h2xxTpdotGQ1fZwsihaIqow337YHQI3q0i\nSqV534Ms+j/tU7X8sq11xFJIeEVG8PASRCwmryUwghFKPlHETQ8jJ+Y8+1asRydi\npsP3B/5Mjhqv/uOK+Vy3zAyIpyDOMtIpOVfjSpCplVRdtSTFWBu9Em7j5I2HMn1w\nsJZnJgXKpybpibGiiTtmnFLOwibmprSu04rsnP4ncdC2XRD4wIjoyA+4PKgX3sCO\nklEzKryWYBmLkJOMDdo52LttP3279s7XrkLEE7ia0fXa2c12EQ0f0DQ1tGUvyVEW\nWmJVccm5bq25AQ0EUw5EzQEIANaPUY04/g7AmYkOMjaCZ6iTp9hB5Rsj/4ee/ln9\nwArzRO9+3eejLWh53FoN1rO+su7tiXJA5YAzVy6tuolrqjM8DBztPxdLBbEi4V+j\n2tK0dATdBQBHEh3OJApO2UBtcjaZBT31zrG9K55D+CrcgIVEHAKY8Cb4kLBkb5wM\nskn+DrASKU0BNIV1qRsxfiUdQHZfSqtp004nrql1lbFMLFEuiY8FZrkkQ9qduixo\nmTT6f34/oiY+Jam3zCK7RDN/OjuWheIPGj/Qbx9JuNiwgX6yRj7OE1tjUx6d8g9y\n0H1fmLJbb3WZZbuuGFnK6qrE3bGeY8+AWaJAZ37wpWh1p0cAEQEAAYkBHwQYAQIA\nCQUCUw5EzQIbDAAKCRBRhS2HNI/8TJntCAClU7TOO/X053eKF1jqNW4A1qpxctVc\nz8eTcY8Om5O4f6a/rfxfNFKn9Qyja/OG1xWNobETy7MiMXYjaa8uUx5iFy6kMVaP\n0BXJ59NLZjMARGw6lVTYDTIvzqqqwLxgliSDfSnqUhubGwvykANPO+93BBx89MRG\nunNoYGXtPlhNFrAsB1VR8+EyKLv2HQtGCPSFBhrjuzH3gxGibNDDdFQLxxuJWepJ\nEK1UbTS4ms0NgZ2Uknqn1WRU1Ki7rE4sTy68iZtWpKQXZEJa0IGnuI2sSINGcXCJ\noEIgXTMyCILo34Fa/C6VCm2WBgz9zZO8/rHIiQm1J5zqz0DrDwKBUM9C\n=LYpS\n-----END PGP PUBLIC KEY BLOCK-----",
		  "trust_signature": "",
		  "source": "HashiCorp",
		  "source_url": "https://www.hashicorp.com/security.html"
		}
	  ]
	}
  }  
`

func TestDownloadJson(t *testing.T) {
	uut := Download{
		Protocols:    []string{"4.0", "5.1"},
		OS:           "linux",
		Arch:         "amd64",
		Filename:     "terraform-provider-random_2.0.0_linux_amd64.zip",
		DownloadURL:  "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_linux_amd64.zip",
		ShaURL:       "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS",
		SignatureURL: "https://releases.hashicorp.com/terraform-provider-random/2.0.0/terraform-provider-random_2.0.0_SHA256SUMS.sig",
		Sha:          "5f9c7aa76b7c34d722fc9123208e26b22d60440cb47150dd04733b9b94f4541a",
		SigningKeys: SigningKeys{
			GpgPublicKeys: []GpgPublicKey{
				{
					KeyID:      "51852D87348FFC4C",
					ASCIIArmor: "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n\nmQENBFMORM0BCADBRyKO1MhCirazOSVwcfTr1xUxjPvfxD3hjUwHtjsOy/bT6p9f\nW2mRPfwnq2JB5As+paL3UGDsSRDnK9KAxQb0NNF4+eVhr/EJ18s3wwXXDMjpIifq\nfIm2WyH3G+aRLTLPIpscUNKDyxFOUbsmgXAmJ46Re1fn8uKxKRHbfa39aeuEYWFA\n3drdL1WoUngvED7f+RnKBK2G6ZEpO+LDovQk19xGjiMTtPJrjMjZJ3QXqPvx5wca\nKSZLr4lMTuoTI/ZXyZy5bD4tShiZz6KcyX27cD70q2iRcEZ0poLKHyEIDAi3TM5k\nSwbbWBFd5RNPOR0qzrb/0p9ksKK48IIfH2FvABEBAAG0K0hhc2hpQ29ycCBTZWN1\ncml0eSA8c2VjdXJpdHlAaGFzaGljb3JwLmNvbT6JATgEEwECACIFAlMORM0CGwMG\nCwkIBwMCBhUIAgkKCwQWAgMBAh4BAheAAAoJEFGFLYc0j/xMyWIIAIPhcVqiQ59n\nJc07gjUX0SWBJAxEG1lKxfzS4Xp+57h2xxTpdotGQ1fZwsihaIqow337YHQI3q0i\nSqV534Ms+j/tU7X8sq11xFJIeEVG8PASRCwmryUwghFKPlHETQ8jJ+Y8+1asRydi\npsP3B/5Mjhqv/uOK+Vy3zAyIpyDOMtIpOVfjSpCplVRdtSTFWBu9Em7j5I2HMn1w\nsJZnJgXKpybpibGiiTtmnFLOwibmprSu04rsnP4ncdC2XRD4wIjoyA+4PKgX3sCO\nklEzKryWYBmLkJOMDdo52LttP3279s7XrkLEE7ia0fXa2c12EQ0f0DQ1tGUvyVEW\nWmJVccm5bq25AQ0EUw5EzQEIANaPUY04/g7AmYkOMjaCZ6iTp9hB5Rsj/4ee/ln9\nwArzRO9+3eejLWh53FoN1rO+su7tiXJA5YAzVy6tuolrqjM8DBztPxdLBbEi4V+j\n2tK0dATdBQBHEh3OJApO2UBtcjaZBT31zrG9K55D+CrcgIVEHAKY8Cb4kLBkb5wM\nskn+DrASKU0BNIV1qRsxfiUdQHZfSqtp004nrql1lbFMLFEuiY8FZrkkQ9qduixo\nmTT6f34/oiY+Jam3zCK7RDN/OjuWheIPGj/Qbx9JuNiwgX6yRj7OE1tjUx6d8g9y\n0H1fmLJbb3WZZbuuGFnK6qrE3bGeY8+AWaJAZ37wpWh1p0cAEQEAAYkBHwQYAQIA\nCQUCUw5EzQIbDAAKCRBRhS2HNI/8TJntCAClU7TOO/X053eKF1jqNW4A1qpxctVc\nz8eTcY8Om5O4f6a/rfxfNFKn9Qyja/OG1xWNobETy7MiMXYjaa8uUx5iFy6kMVaP\n0BXJ59NLZjMARGw6lVTYDTIvzqqqwLxgliSDfSnqUhubGwvykANPO+93BBx89MRG\nunNoYGXtPlhNFrAsB1VR8+EyKLv2HQtGCPSFBhrjuzH3gxGibNDDdFQLxxuJWepJ\nEK1UbTS4ms0NgZ2Uknqn1WRU1Ki7rE4sTy68iZtWpKQXZEJa0IGnuI2sSINGcXCJ\noEIgXTMyCILo34Fa/C6VCm2WBgz9zZO8/rHIiQm1J5zqz0DrDwKBUM9C\n=LYpS\n-----END PGP PUBLIC KEY BLOCK-----",
					Source:     "HashiCorp",
					SourceURL:  "https://www.hashicorp.com/security.html",
				},
			},
		},
	}

	t.Run("Format", func(t *testing.T) {
		formattedUut, err := json.Marshal(&uut)
		require.NoError(t, err)

		// To avoid having to deal with trivial formatting & whitespace
		// differences we will parse the generated JSON into a generic
		// nested key-value map, and compare it against a similar map
		// generated from the expected JSON.

		var expected map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(validDownloadText), &expected))

		var actual map[string]interface{}
		require.NoError(t, json.Unmarshal(formattedUut, &actual))

		require.Equal(t, expected, actual)
	})

	t.Run("Parsing", func(t *testing.T) {
		var parsedDownload Download
		require.NoError(t, json.Unmarshal([]byte(validDownloadText), &parsedDownload))
		require.Equal(t, uut, parsedDownload)
	})
}
