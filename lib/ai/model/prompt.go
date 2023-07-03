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

package model

import (
	"fmt"
	"strings"
)

var observationPrefix = "Observation: "
var thoughtPrefix = "Thought: "

const PromptSummarizeTitle = `You will be given a message. Create a short summary of that message.
Respond only with summary, nothing else.`

const PromptSummarizeCommand = `You will be given a chat history and a command output. Based on the history context, extract relevant information from the command output and write a short summary of the command output.
Respond only with summary, nothing else.`

const InitialAIResponse = `Hey, I'm Teleport - a powerful tool that can assist you in managing your Teleport cluster via OpenAI GPT-4.`

func PromptCharacter(username string) string {
	return fmt.Sprintf(`You are Teleport, a tool that users can use to connect to Linux servers and run relevant commands, as well as have a conversation.
A Teleport cluster is a connectivity layer that allows access to a set of servers. Servers may also be referred to as nodes.
Nodes sometimes have labels such as "production" and "staging" assigned to them. Labels are used to group nodes together.
You will engage in professional conversation with the user and help accomplish tasks such as executing tasks
within the cluster or answering relevant questions about Teleport, Linux or the cluster itself.

You possess advanced capabilities to think and reason in multiple steps and use the available tools to accomplish the task at hand in a way a human would expect you to.

You are not permitted to engage in conversation that is not related to Teleport, Linux or the cluster itself.
If this user asks such an unrelated question, you must concisely respond that it is beyond your scope of knowledge.

You are talking to %v.`, username)
}

func conversationParserFormatInstructionsPrompt(toolnames []string) string {
	return fmt.Sprintf(`RESPONSE FORMAT INSTRUCTIONS
----------------------------

When responding to me, please output a response in one of two formats:

**Option 1:**
Use this if you want the human to use a tool.
Markdown code snippet formatted in the following schema:

%vjson
{
	"action": string \\ The action to take. Must be one of %v
	"action_input": string \\ The input to the action
	"reasoning": string \\ A short consice thought with the reasoning for taking this action
}
%v

**Option #2:**
Use this if you want to respond directly to the human or you want to ask the human a question to gather more information.
You should avoid asking too many questions when you have other options available to you as it may be perceived as annoying.
But asking is far better than guessing or making assumptions.
Text with the hardcoded header %v followed by your response as below:

%v
YOUR RESPONSE HERE`, "```", toolnames, "```", finalResponseHeader, finalResponseHeader,
	)
}

func conversationToolUsePrompt(tools string, formatInstructions string, userInput string) string {
	return fmt.Sprintf(`TOOLS
------
Assistant can ask the user to use tools to look up information that may be helpful in answering the users original question. The tools the human can use are:

%v

%v

USER'S INPUT
--------------------
Here is the user's input (remember to respond with a markdown code snippet of a json blob with a single action, and NOTHING else):

%v`, tools, formatInstructions, userInput)
}

func conversationToolResponse(toolResponse string) string {
	return fmt.Sprintf(`TOOL RESPONSE: 
---------------------

%v

USER'S INPUT
--------------------

Okay, so what is the response to my last comment? If using information obtained from the tools you must mention it explicitly without mentioning the tool names - I have forgotten all TOOL RESPONSES! Remember to respond with a markdown code snippet of a json blob with a single action, and NOTHING else.`, toolResponse)
}

func ConversationCommandResult(result map[string][]byte) string {
	var message strings.Builder
	for node, output := range result {
		message.WriteString(fmt.Sprintf(`Command ran on node "%s" and produced the following output:\n`, node))
		message.WriteString(string(output))
		message.WriteString("\n")
	}
	message.WriteString("Based on the chat history, extract relevant information out of the command output and write a summary.")
	return message.String()
}

func MessageClassificationPrompt(classes map[string]string) string {
	var classList strings.Builder
	for name, description := range classes {
		classList.WriteString(fmt.Sprintf("- `%s` (%s)\n", name, description))
	}

	return fmt.Sprintf(`Teleport is a tool that provides access to servers, kubernetes clusters, databases, and applications. All connected Teleport resources are called a cluster. Server resources might be called nodes.

Classify the provided message between the following categories:

%v

Answer only with the category name. Nothing else.`, classList.String())
}
