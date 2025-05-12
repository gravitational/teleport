/*
Copyright 2023 Gravitational, Inc.

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

package img

import (
	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/awslabs/amazon-ecr-credential-helper/ecr-login/api"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/gravitational/trace"
)

type CredentialSource string

const (
	DockerCredentialSource = "docker"
	AmazonCredentialSource = "aws"
	GoogleCredentialSource = "google"
	NoCredentialSource     = "none"
)

// GetKeychain builds a ggcr keychain for image pulling and returns it.
// We could attempt to autodetect or build a multi-keychain but ECR login
// attempts take a lot of time and log errors. As most users don't need registry
// auth, it's acceptable to have them do an extra step and specify which auth
// they need.
func GetKeychain(credSource string) (authn.Keychain, error) {
	switch credSource {
	case DockerCredentialSource:
		return authn.DefaultKeychain, nil
	case AmazonCredentialSource:
		return authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithClientFactory(api.DefaultClientFactory{}))), nil
	case GoogleCredentialSource:
		return google.Keychain, nil
	case NoCredentialSource:
		return nil, nil
	default:
		return nil, trace.BadParameter("credential source '%s' not recognized", credSource)
	}
}
