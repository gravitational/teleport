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

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/gen/proto/go/assist/v1"
	"github.com/gravitational/teleport/api/types"
	embeddinglib "github.com/gravitational/teleport/lib/ai/embedding"
	modeloutput "github.com/gravitational/teleport/lib/ai/model/output"
	"github.com/gravitational/teleport/lib/services"
)

type EmbeddingRetrievalTool struct{}

type EmbeddingRetrievalToolInput struct {
	Question string `json:"question"`
}

// tryNodeLookupFromProxyCache checks how many nodes the user has access to by
// hitting the proxy cache.  If the user has access to less than
// maxEmbeddingsPerLookup, the returned boolean indicates the lookup is
// successful and the result can be used. If the boolean is false, the caller
// must not use the returned result and perform a Node lookup via other means
// (embeddings lookup).
func (e *EmbeddingRetrievalTool) tryNodeLookupFromProxyCache(ctx context.Context, toolCtx *ToolContext) (bool, string, error) {
	nodes := toolCtx.NodeWatcher.GetNodes(ctx, func(node services.Node) bool {
		err := toolCtx.CheckAccess(node, services.AccessState{MFAVerified: true})
		return err == nil
	})
	if len(nodes) == 0 || len(nodes) > maxEmbeddingsPerLookup {
		return false, "", nil
	}
	sb := strings.Builder{}
	for _, node := range nodes {
		data, err := embeddinglib.SerializeNode(node)
		if err != nil {
			return false, "", trace.Wrap(err)
		}
		sb.Write(data)
		sb.WriteString("\n")
	}
	return true, sb.String(), nil
}

func (e *EmbeddingRetrievalTool) Run(ctx context.Context, toolCtx *ToolContext, input string) (string, error) {
	inputCmd, outErr := e.parseInput(input)
	if outErr == nil {
		// If we failed to parse the input, we can still send the payload for embedding retrieval.
		// In most cases, we will still get some sensible results.
		// If we parsed the input successfully, we should use the parsed input instead.
		input = inputCmd.Question
	}
	log.Tracef("embedding retrieval input: %v", input)

	// Threshold to avoid looping over all nodes on large clusters
	if toolCtx.NodeWatcher != nil && toolCtx.NodeWatcher.NodeCount() < proxyLookupClusterMaxSize {
		ok, result, err := e.tryNodeLookupFromProxyCache(ctx, toolCtx)
		if err != nil {
			return "", trace.Wrap(err)
		}
		if ok {
			return result, nil
		}
	}

	resp, err := toolCtx.GetAssistantEmbeddings(ctx, &assist.GetAssistantEmbeddingsRequest{
		Username: toolCtx.User,
		Kind:     types.KindNode, // currently only node embeddings are supported
		Limit:    maxEmbeddingsPerLookup,
		Query:    input,
	})
	if err != nil {
		return "", trace.Wrap(err)
	}

	sb := strings.Builder{}
	for _, embedding := range resp.Embeddings {
		sb.WriteString(embedding.Content)
		sb.WriteString("\n")
	}

	log.Tracef("embedding retrieval: %v", sb.String())

	if sb.Len() == 0 {
		// Either no nodes are connected, embedding process hasn't started yet, or
		// the user doesn't have access to any resources.
		return "Didn't find any nodes matching the query", nil
	}

	return sb.String(), nil
}

func (e *EmbeddingRetrievalTool) Name() string {
	return "Nodes names and labels retrieval"
}

func (e *EmbeddingRetrievalTool) Description() string {
	return fmt.Sprintf(`Ask about existing remote nodes that user has access to fetch node names or/and set of labels. 
Always use this capability before returning generating any command. Do not assume that the user has access to any nodes. Returning a command without checking for access will result in an error.
Always prefer to use labler rather than node names.
The input must be a JSON object with the following schema:
%vjson
{
	"question": string \\ Question about the available remote nodes
}
%v
`, "```", "```")
}

func (*EmbeddingRetrievalTool) parseInput(input string) (*EmbeddingRetrievalToolInput, error) {
	output, err := modeloutput.ParseJSONFromModel[EmbeddingRetrievalToolInput](input)
	if err != nil {
		return nil, err
	}

	if len(output.Question) == 0 {
		return nil, modeloutput.NewInvalidOutputError(
			"embedding retrieval: missing question",
			"question must be non-empty",
		)
	}

	return &output, nil
}
