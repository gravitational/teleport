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

package common

import (
	"context"
)

// MessagingBot is a generic interface with all methods required to send notifications through a messaging service.
// A messaging bot contains an API client to send/edit messages and is able to resolve a Recipient from a string.
// Implementing this interface allows to leverage BaseApp logic without customization.
type MessagingBot interface {
	// CheckHealth checks if the bot can connect to its messaging service
	CheckHealth(ctx context.Context) error
	// FetchRecipient fetches recipient data from the messaging service API. It can also be used to check and initialize
	// a communication channel (e.g. MsTeams needs to install the app for the user before being able to send
	// notifications)
	FetchRecipient(ctx context.Context, recipient string) (*Recipient, error)
	// SupportedApps are the apps supported by this bot.
	SupportedApps() []App
}
