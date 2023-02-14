/*
Copyright 2021-2022 Gravitational, Inc.

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

package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/schemagen"
)

const groupName = "resources.teleport.dev"

/*
Fields that we are ignoring when creating a CRD
Each entry represents the ignore fields using the resource name as the version

One of the reasons to ignore fields those fields is because they are readonly in Teleport
CRD do not support readonly logic
This should be removed when the following feature is implemented
https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#transition-rules
*/
var config = []schemagen.ParseResourceOptions{
	schemagen.ParseResourceOptions{
		Name: "UserV2",
		IgnoredFields: []string{
			"LocalAuth",
			"Expires",
			"CreatedBy",
			"Status",
		},
	},
	// Use RoleV6 spec but override the version to V5. This will generate
	// crd based on RoleV6 but with resource version for v5.
	schemagen.ParseResourceOptions{
		Name:            "RoleV6",
		VersionOverride: types.V5,
	},
	schemagen.ParseResourceOptions{
		Name: "RoleV6",
	},
	schemagen.ParseResourceOptions{
		Name: "SAMLConnectorV2",
	},
	schemagen.ParseResourceOptions{
		Name: "OIDCConnectorV3",
	},
	schemagen.ParseResourceOptions{
		Name:          "GithubConnectorV3",
		IgnoredFields: []string{"TeamsToLogins"}, // Deprecated field, removed since v11
	},
}

func main() {
	log.SetLevel(log.DebugLevel)
	log.SetOutput(os.Stderr)

	if err := schemagen.RunPlugin(config, GenerateCRD); err != nil {
		log.WithError(err).Error("Failed to generate schema")
		os.Exit(-1)
	}
}

func GenerateCRD(c *schemagen.SchemaCollection) ([]*schemagen.TransformedFile, error) {
	var files []*schemagen.TransformedFile
	for _, root := range c.Roots {
		crd := CustomResourceDefinition(root, groupName)
		data, err := yaml.Marshal(crd)
		if err != nil {
			return nil, err
		}
		files = append(files, &schemagen.TransformedFile{
			Name:    fmt.Sprintf("%s_%s.yaml", groupName, root.PluralName),
			Content: string(data),
		})
	}

	return files, nil
}
