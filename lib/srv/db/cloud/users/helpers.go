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

package users

import (
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/secrets"
	"github.com/gravitational/teleport/lib/utils"
)

// lookupEntry is the entry value for lookupMap.
type lookupEntry struct {
	database types.Database
	users    []User
}

// lookupMap is a mapping of database names to their managed users.
type lookupMap struct {
	byName map[string]lookupEntry
	mu     sync.RWMutex
}

// newLookupMap creates a new lookup map.
func newLookupMap() *lookupMap {
	return &lookupMap{
		byName: make(map[string]lookupEntry),
	}
}

// getDatabaseUser finds a database user by database username.
func (m *lookupMap) getDatabaseUser(database types.Database, username string) (User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.byName[database.GetName()].users {
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
		m.byName[database.GetName()] = lookupEntry{
			database: database,
			users:    users,
		}
	} else {
		delete(m.byName, database.GetName())

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

func (m *lookupMap) removeIfURIChanged(database types.Database) {
	m.mu.Lock()
	defer m.mu.Unlock()

	current, ok := m.byName[database.GetName()]
	if !ok || current.database.GetURI() == database.GetURI() {
		return
	}
	delete(m.byName, database.GetName())
}

// removeUnusedDatabases removes unused databases by comparing with provided
// active databases.
func (m *lookupMap) removeUnusedDatabases(activeDatabases types.Databases) {
	m.mu.Lock()
	defer m.mu.Unlock()

	activeDatabasesMap := activeDatabases.ToMap()
	for databaseName := range m.byName {
		if _, isActive := activeDatabasesMap[databaseName]; !isActive {
			delete(m.byName, databaseName)
		}
	}
}

// usersByID returns a map of users by their IDs.
func (m *lookupMap) usersByID() map[string]User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usersByID := make(map[string]User)
	for _, entry := range m.byName {
		for _, user := range entry.users {
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
func newSecretStore(ss types.SecretStore, client secrets.SecretsManagerClient, clusterName string) (secrets.Secrets, error) {
	return secrets.NewAWSSecretsManager(secrets.AWSSecretsManagerConfig{
		KeyPrefix:   ss.KeyPrefix,
		KMSKeyID:    ss.KMSKeyID,
		Client:      client,
		ClusterName: clusterName,
	})
}
