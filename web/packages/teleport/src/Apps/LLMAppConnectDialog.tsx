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

import { ReactNode } from 'react';

import { Box, ButtonSecondary, Stack, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { generateTshLoginCommand } from 'teleport/lib/util';
import { App, LLMFormat, LLMProvider } from 'teleport/services/apps';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

/**
 * Port the instructions pin the local proxy to, so Step 2 and Step 3 can
 * reference a concrete address instead of a randomized one.
 */
const LOCAL_PROXY_PORT = '3000';
const localProxyURL = `http://127.0.0.1:${LOCAL_PROXY_PORT}`;

const apiKeyComment =
  'Any non-empty value works; Teleport swaps in the real key.';

type EnvLine = { text: string; comment?: string };

type ProviderSpec = {
  /** Bold heading naming the clients these instructions apply to. */
  clientLabel: string;
  /** Optional comment shown above the environment variables. */
  description?: string;
  /** `export <variable>=<value>` lines to set before launching the client. */
  envLines: EnvLine[];
  /** Prose introducing the command that launches the client. */
  runLabel: string;
  /** Command that launches (and, for Codex, configures) the client. */
  runCommand: string;
};

/**
 * getProviderSpec returns the Step 3 instructions for an inference endpoint.
 * They depend on the API format and the provider eg. Codex ignores base-URL
 * environment variables so it needs the address inline, and the inference endpoint
 * served by Bedrock needs an extra environment variable to disable beta features.
 */
function getProviderSpec(
  format: LLMFormat,
  provider: LLMProvider | undefined
): ProviderSpec {
  if (format === 'openai') {
    return {
      clientLabel: 'Codex, or any OpenAI client',
      description: 'Most OpenAI clients read these environment variables:',
      envLines: [
        { text: `export OPENAI_BASE_URL=${localProxyURL}/v1` },
        { text: 'export OPENAI_API_KEY=teleport', comment: apiKeyComment },
      ],
      runLabel:
        'Codex ignores those variables, so start it with the address inline instead:',
      runCommand: `codex -c openai_base_url=${localProxyURL}/v1`,
    };
  }

  const envLines: EnvLine[] = [
    { text: `export ANTHROPIC_BASE_URL=${localProxyURL}` },
    { text: 'export ANTHROPIC_API_KEY=teleport', comment: apiKeyComment },
  ];
  if (provider === 'bedrock') {
    envLines.push({
      text: 'export CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1',
      comment: 'Required when the endpoint is served by Amazon Bedrock.',
    });
  }
  return {
    clientLabel: 'Claude Code, Claude Agent SDK, or any Anthropic client',
    envLines,
    runLabel: 'Then run Claude Code as usual:',
    runCommand: 'claude',
  };
}

export function LLMAppConnectDialog(props: { app: App; onClose: () => void }) {
  const { app } = props;
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const { username, authType } = ctx.storeUser.state;
  const accessRequestId = ctx.storeUser.getAccessRequestId();

  const format: LLMFormat = app.llmFormat === 'openai' ? 'openai' : 'anthropic';
  const spec = getProviderSpec(format, app.llmProvider);

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '600px',
        width: '100%',
      })}
      disableEscapeKeyDown={false}
      onClose={props.onClose}
      open={true}
    >
      <DialogHeader mb={4}>
        <DialogTitle>Use "{app.name}" inference endpoints</DialogTitle>
      </DialogHeader>

      <DialogContent>
        <Stack gap={4} fullWidth>
          <Step number={1} title="Log in to Teleport">
            <Command
              command={generateTshLoginCommand({
                authType,
                username,
                clusterId,
                accessRequestId,
              })}
            />
          </Step>

          <Step
            number={2}
            title="Start a local proxy for the inference endpoint"
          >
            <Command
              command={`tsh proxy app ${app.name} --port ${LOCAL_PROXY_PORT}`}
            />
            <Box>
              This listens on {localProxyURL}. Every request is authenticated
              and audited by Teleport, which also injects the provider API key -
              so no real key is needed locally.
            </Box>
          </Step>

          <Step number={3} title="Point your LLM client at the local proxy">
            <ProviderInstructions spec={spec} />
          </Step>
        </Stack>
      </DialogContent>

      <DialogFooter>
        <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

function Step(props: { number: number; title: string; children: ReactNode }) {
  return (
    <Stack fullWidth gap={2}>
      <Text>
        <Text bold as="span">
          Step {props.number}
        </Text>
        {` - ${props.title}`}
      </Text>
      {props.children}
    </Stack>
  );
}

function ProviderInstructions({ spec }: { spec: ProviderSpec }) {
  return (
    <>
      <Box>
        <Text bold>{spec.clientLabel}</Text>
      </Box>
      {spec.description && <Box>{spec.description}</Box>}
      <TextSelectCopyMulti lines={spec.envLines} />
      <Box>{spec.runLabel}</Box>
      <Command command={spec.runCommand} />
    </>
  );
}

function Command({ command }: { command: string }) {
  return <TextSelectCopyMulti lines={[{ text: command }]} />;
}
