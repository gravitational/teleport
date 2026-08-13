/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { Box, Flex, Text } from 'design';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

type LlmEnvLine = { text: string; comment?: string };

export type LlmSpec = {
  /** Provider name shown in the title, e.g. "Anthropic". */
  name: string;
  /** Client named in the instructions, e.g. "Anthropic/OpenAI" client. */
  clientLabel: string;
  /** `export <variable>=<value>` lines to set before launching the client. */
  envLines?: LlmEnvLine[];
  /** Optional note shown above the run command. */
  runNote?: string;
  /** Command that launches the client. */
  runCommand: string;
};

/**
 * getLlmSpec returns the client-specific instructions for the running proxy.
 * They depend on the API format and provider, mirroring the web UI: Codex
 * ignores base-URL environment variables so it needs the address inline, and
 * an endpoint served by Bedrock needs provider-specific configuration.
 *
 * Any non-OpenAI format (including unset) gets Anthropic instructions,
 * matching LLMAppConnectDialog in the web UI.
 */
export function getLlmSpec(
  llmFormat: string,
  llmProvider: string,
  address: string
): LlmSpec {
  if (llmFormat === 'openai') {
    if (llmProvider === 'bedrock') {
      return {
        name: 'OpenAI',
        clientLabel: 'OpenAI client',
        runNote:
          'This endpoint is served by Amazon Bedrock, so Codex must use its Bedrock model provider (requires Codex 0.145.0+):',
        runCommand:
          `codex -c model_providers.amazon-bedrock.base_url=${address} ` +
          `-c model_providers.amazon-bedrock.auth.command=cat ` +
          `-c model_provider=amazon-bedrock`,
      };
    }
    return {
      name: 'OpenAI',
      clientLabel: 'OpenAI client',
      envLines: [
        { text: `export OPENAI_BASE_URL=${address}/v1` },
        { text: 'export OPENAI_API_KEY=teleport' },
      ],
      runNote:
        'Codex ignores the base-URL variable, so pass the address inline:',
      runCommand: `codex -c openai_base_url=${address}/v1`,
    };
  }

  const envLines: LlmEnvLine[] = [
    { text: `export ANTHROPIC_BASE_URL=${address}` },
    { text: 'export ANTHROPIC_API_KEY=teleport' },
  ];
  if (llmProvider === 'bedrock') {
    envLines.push({
      text: 'export CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1',
      comment: 'Required when the endpoint is served by Amazon Bedrock.',
    });
  }
  return {
    name: 'Anthropic',
    clientLabel: 'Anthropic client (ex. Claude Code, Claude Agent SDK)',
    envLines,
    runCommand: 'claude',
  };
}

/**
 * LlmInstructions tells the user how to point their LLM client at the running
 * local proxy. Teleport authenticates and audits every request and injects the
 * provider API key, so no real key is needed locally.
 */
export function LlmInstructions({ spec }: { spec: LlmSpec }) {
  return (
    <Flex flexDirection="column" gap={2}>
      <Text>
        Point your {spec.clientLabel} at the local proxy. Every request is
        authenticated and audited by Teleport, which also injects the provider
        API key - so no real key is needed locally.
      </Text>
      {spec.envLines?.map((line, index) => (
        <Box key={index}>
          {line.comment && (
            <Text color="text.slightlyMuted" mb={1}>
              {line.comment}
            </Text>
          )}
          <TextSelectCopy text={line.text} bash={false} />
        </Box>
      ))}
      {spec.runNote && <Text>{spec.runNote}</Text>}
      <TextSelectCopy text={spec.runCommand} bash={false} />
    </Flex>
  );
}
