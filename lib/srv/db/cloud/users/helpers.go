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

package users

import (
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud"
	"github.com/gravitational/teleport/lib/srv/db/secrets"
	"github.com/gravitational/teleport/lib/utils"
)

// lookupMap is a mapping of database objects to their managed users.
type lookupMap struct {
	byDatabase map[types.Database][]User
	mu         sync.RWMutex
}

// newLookupMap creates a new lookup map.
func newLookupMap() *lookupMap {
	return &lookupMap{
		byDatabase: make(map[types.Database][]User),
	}
}

// getDatabaseUser finds a database user by database username.
func (m *lookupMap) getDatabaseUser(database types.Database, username string) (User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.byDatabase[database] {
		if user.GetDatabaseUsername() == username {
			return user, true
		}
	}
	return nil, false
}

// setDatabaseUsers sets the database users for future lookups.
func (m *lookupMap) setDatabaseUsers(database types.Database, users []User) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(users) > 0 {
		m.byDatabase[database] = users
	} else {
		delete(m.byDatabase, database)

		// Short circuit.
		if len(database.GetManagedUsers()) == 0 {
			return
		}
	}

	// Update database resource.
	var usernames []string
	for _, user := range users {
		usernames = append(usernames, user.GetDatabaseUsername())
	}
	database.SetManagedUsers(usernames)
}

// removeUnusedDatabases removes unused databases by comparing with provided
// active databases.
func (m *lookupMap) removeUnusedDatabases(activeDatabases types.Databases) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for database := range m.byDatabase {
		if isActive := findDatabase(activeDatabases, database); !isActive {
			delete(m.byDatabase, database)
		}
	}
}

// findDatabase finds the database object in provided list of databases.
func findDatabase(databases types.Databases, database types.Database) bool {
	for i := range databases {
		if databases[i] == database {
			return true
		}
	}
	return false
}

// usersByID returns a map of users by their IDs.
func (m *lookupMap) usersByID() map[string]User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usersByID := make(map[string]User)
	for _, users := range m.byDatabase {
		for _, user := range users {
			usersByID[user.GetID()] = user
		}
	}
	return usersByID
}

// secretKeyFromAWSARN creates a secret key with provided ARN.
func secretKeyFromAWSARN(inputARN string) (string, error) {
	// Example ElastiCache User ARN looks like this:
	// arn:aws:elasticache:<region>:<account-id>:user:<user-id>
	//
	// Make an unique secret key like this:
	// elasticache/<region>/<account-id>/user/<user-id>
	parsed, err := arn.Parse(inputARN)
	if err != nil {
		return "", trace.BadParameter(err.Error())
	}
	return secrets.Key(
		parsed.Service,
		parsed.Region,
		parsed.AccountID,
		strings.ReplaceAll(parsed.Resource, ":", "/"),
	), nil
}

// genRandomPassword generate a random password with provided length.
func genRandomPassword(length int) (string, error) {
	if length <= 0 {
		return "", trace.BadParameter("invalid random value length")
	}

	// Hex generated from CryptoRandomHex is twice of the input.
	hex, err := utils.CryptoRandomHex((length + 1) / 2)
	if err != nil {
		return "", trace.Wrap(err)
	} else if len(hex) < length {
		return "", trace.CompareFailed("generated hex is too short")
	}
	return hex[:length], nil
}

// newSecretStore create a new secrets store helper for provided database.
func newSecretStore(database types.Database, clients cloud.Clients) (secrets.Secrets, error) {
	secretStoreConfig := database.GetSecretStore()

	client, err := clients.GetAWSSecretsManagerClient(database.GetAWS().Region)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets.NewAWSSecretsManager(secrets.AWSSecretsManagerConfig{
		KeyPrefix: secretStoreConfig.KeyPrefix,
		KMSKeyID:  secretStoreConfig.KMSKeyID,
		Client:    client,
	})
}
