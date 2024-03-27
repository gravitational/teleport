// Copyright 2023 Gravitational, Inc
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

package msapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gravitational/trace"
)

const (
	botFrameworkAuthScope     = "https://api.botframework.com/.default"
	botFrameworkBaseURL       = "https://smba.trafficmanager.net"
	botFrameworkDefaultRegion = "emea"
	botFrameworkVersion       = "v3"

	// https://hackandchat.com/teams-proactive-messaging/
	botDesignator = "29:"
)

// BotFrameworkClient represents client to MS Graph API
type BotFrameworkClient struct {
	Client
}

// PostActivityResponse represents json response with a single id field
type PostActivityResponse struct {
	ID string `json:"id"`
}

// botError represents MS Graph error
type botError struct {
	E struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// postMessagePayload represents utility struct for PostAdaptiveCard payload
type postMessagePayload struct {
	Type        string                         `json:"type"`
	From        postMessagePayloadFrom         `json:"from"`
	Attachments []postMessagePayloadAttachment `json:"attachments"`
}

type postMessagePayloadFrom struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type postMessagePayloadAttachment struct {
	ContentType string          `json:"contentType"`
	Content     json.RawMessage `json:"content"`
}

// Error returns error string
func (e *botError) Error() string {
	return e.E.Code + " " + e.E.Message
}

// NewBotFrameworkClient creates MS Graph API client
func NewBotFrameworkClient(config Config) *BotFrameworkClient {
	region := config.Region
	if region == "" {
		region = botFrameworkDefaultRegion
	}

	baseURL := config.url.botFrameworkBaseURL
	if baseURL == "" {
		baseURL = botFrameworkBaseURL
	}

	baseURL = baseURL + "/" + region + "/" + botFrameworkVersion

	return &BotFrameworkClient{
		Client: Client{
			token:   tokenWithTTL{scope: botFrameworkAuthScope, baseURL: config.url.tokenBaseURL},
			baseURL: baseURL,
			config:  config,
		},
	}
}

// PostAdaptiveCardActivity sends an activity to the chat, content is AdaptiveCard
func (c *BotFrameworkClient) PostAdaptiveCardActivity(ctx context.Context, botID, chatID, card, updateID string) (string, error) {
	m := postMessagePayload{
		Type: "message",
		From: postMessagePayloadFrom{
			ID:   botDesignator + botID,
			Name: "TeleBot",
		},
		Attachments: []postMessagePayloadAttachment{{
			ContentType: "application/vnd.microsoft.card.adaptive",
			Content:     []byte(card),
		}},
	}

	body, err := json.MarshalIndent(&m, "", "    ")
	if err != nil {
		return "", trace.Wrap(err)
	}

	id := PostActivityResponse{}

	meth := http.MethodPost
	status := http.StatusCreated
	if updateID != "" {
		meth = http.MethodPut
		status = http.StatusOK
	}

	request := request{
		Method:      meth,
		Path:        "conversations/" + chatID + "/activities/" + updateID,
		Body:        string(body),
		Response:    &id,
		SuccessCode: status,
		Err:         &botError{},
	}

	err = c.request(ctx, request)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return id.ID, nil
}
