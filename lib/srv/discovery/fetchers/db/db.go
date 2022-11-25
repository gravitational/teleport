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

package db

import (
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
)

// filterDatabasesByLabels filters input databases with provided labels and
// converts the databases to types.ResourcesWithLabels.
func filterDatabasesByLabels(databases types.Databases, labels types.Labels, log logrus.FieldLogger) types.ResourcesWithLabels {
	var matchedDatabases types.Databases
	for _, database := range databases {
		match, _, err := services.MatchLabels(labels, database.GetAllLabels())
		if err != nil {
			log.Warnf("Failed to match %v against selector: %v.", database, err)
		} else if match {
			matchedDatabases = append(matchedDatabases, database)
		} else {
			log.Debugf("%v doesn't match selector.", database)
		}
	}
	return matchedDatabases.AsResources()
}
