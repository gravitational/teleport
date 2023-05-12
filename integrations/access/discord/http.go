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

package discord

import (
	"net/http"

	"github.com/go-resty/resty/v2"
)

const discordAPIURL = "https://discord.com/api/"

func makeSlackClient(apiURL string) *resty.Client {
	return resty.
		NewWithClient(&http.Client{
			Timeout: discordHTTPTimeout,
			Transport: &http.Transport{
				MaxConnsPerHost:     discordMaxConns,
				MaxIdleConnsPerHost: discordMaxConns,
			},
		}).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetBaseURL(apiURL)
}
