// Copyright 2024 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package msteams

import (
	"context"
	"fmt"
	"time"

	cards "github.com/DanielTitkov/go-adaptive-cards"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/integrations/lib"
	"github.com/gravitational/teleport/integrations/lib/plugindata"
)

// Validate installs the application for a user if required and sends the Hello, world! message
func Validate(configPath, recipient string) error {

	ctx := context.Background()
	b, _, err := loadConfig(configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	err = checkApp(ctx, b)
	if err != nil {
		return trace.Wrap(err)
	}

	if lib.IsEmail(recipient) {
		userID, err := b.GetUserIDByEmail(context.Background(), recipient)
		if trace.IsNotFound(err) {
			fmt.Printf(" - User %v not found! Try to use user ID instead\n", recipient)
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}

		fmt.Printf(" - User %v found: %v\n", recipient, userID)

		recipient = userID
	}

	recipientData, err := b.FetchRecipient(context.Background(), recipient)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(" - Application installation ID for recipient: %v\n", recipientData.App.ID)
	fmt.Printf(" - Chat ID for recipient: %v\n", recipientData.Chat.ID)
	fmt.Printf(" - Chat web URL: %v\n", recipientData.Chat.WebURL)

	card := cards.New([]cards.Node{
		&cards.TextBlock{
			Text: "Hello, world!",
			Size: "large",
		},
		&cards.TextBlock{
			Text: "*Sincerely yours,*",
		},
		&cards.TextBlock{
			Text: "Teleport Bot!",
		},
	}, []cards.Node{}).
		WithSchema(cards.DefaultSchema).
		WithVersion(cards.Version12)

	body, err := card.StringIndent("", "    ")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println(" - Sending the message...")

	id, err := b.PostAdaptiveCardActivity(context.Background(), recipient, body, "")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(" - Message sent, ID: %v\n", id)

	data := plugindata.AccessRequestData{
		User:          "foo",
		Roles:         []string{"editor"},
		RequestReason: "Example request posted by 'validate' command.",
		ReviewsCount:  1,
	}

	reviews := []types.AccessReview{
		{
			Author:        "bar",
			Roles:         []string{"reviewer"},
			ProposedState: types.RequestState_APPROVED,
			Reason:        "Looks fine",
			Created:       time.Now(),
		},
		{
			Author:        "baz",
			Roles:         []string{"reviewer"},
			ProposedState: types.RequestState_DENIED,
			Reason:        "Not good",
			Created:       time.Now(),
		},
	}

	body, err = BuildCard(uuid.NewString(), nil, "local-cluster", data, reviews)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = b.PostAdaptiveCardActivity(context.Background(), recipient, body, "")
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Println()
	fmt.Println("Check your MS Teams!")

	return nil
}

func loadConfig(configPath string) (*Bot, *Config, error) {
	c, err := LoadConfig(configPath)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	fmt.Printf(" - Checking application %v status...\n", c.MSAPI.TeamsAppID)

	b, err := NewBot(c.MSAPI, "local", "")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return b, c, nil
}

func checkApp(ctx context.Context, b *Bot) error {
	teamApp, err := b.GetTeamsApp(ctx)
	if trace.IsNotFound(err) {
		fmt.Printf("Application %v not found in the org app store. Please, ensure that you have the application uploaded and installed for your team.", b.Config.TeamsAppID)
		return trace.Wrap(err)
	} else if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(" - Application found in the team app store (internal ID: %v)\n", teamApp.ID)
	return nil
}
