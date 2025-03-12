/*
 * *
 *  * Teleport
 *  * Copyright (C) 2024 Gravitational, Inc.
 *  *
 *  * This program is free software: you can redistribute it and/or modify
 *  * it under the terms of the GNU Affero General Public License as published by
 *  * the Free Software Foundation, either version 3 of the License, or
 *  * (at your option) any later version.
 *  *
 *  * This program is distributed in the hope that it will be useful,
 *  * but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  * GNU Affero General Public License for more details.
 *  *
 *  * You should have received a copy of the GNU Affero General Public License
 *  * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package summary

import (
	"context"
	"fmt"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"
	"github.com/sashabaranov/go-openai"
	"os"
)

func GenerateSummary(sessionID session.ID) (string, error) {
	client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))

	prompt := fmt.Sprintf(
		`Summarize this Teleport session recording:

Session ID: %s
Type: %s

Session Content:
%s

Please provide a concise summary of what commands were executed, their purpose, and any notable details in 3-5 sentences.
If this session is a Kubernetes session, note the relevant pod/cluster/namespace if present.`,
		sessionID,
		sessionType(sessionID),
		sessionContent(sessionID),
	)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: prompt,
				},
			},
		},
	)

	if err != nil {
		return "", trace.Wrap(err)
	}

	return resp.Choices[0].Message.Content, nil
}

func sessionType(sessionID session.ID) string {
	return "ssh"
}

func sessionContent(sessionID session.ID) string {
	return `
> echo 1
1
> cat ~/.ssh/authorized_keys
> $?
0
> for i in /sys/class/leds/*; do 
rm -rf /var/lib/setter/states/$i > /dev/null
ln -s /sys/class/leds/$i/brightness /var/lib/setter/states/$i > /dev/null
done
> ls /var/lib/setter/states
a
b
c
> $?
0
`
}
