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
import {
  TextSelectCopy,
  TextSelectCopyMulti,
} from 'shared/components/TextSelectCopy';

import { generateTshLoginCommand } from 'teleport/lib/util';
import { App, LLMFormat } from 'teleport/services/apps';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

/**
 * Port the instructions pin the local proxy to, so Step 2 and Step 3 can
 * reference a concrete address instead of a randomized one.
 */
const LOCAL_PROXY_PORT = '3000';
const localProxyURL = `http://127.0.0.1:${LOCAL_PROXY_PORT}`;

/** Per-provider Step 3 instructions. */
type ProviderSpec = {
  /** Bold heading naming the clients these instructions apply to. */
  clientLabel: string;
  /** `export <VAR>=<url>` line pointing the client at the local proxy. */
  baseUrlEnv: string;
  /** `export <VAR>=<placeholder>` line for the (unused) client-side API key. */
  apiKeyEnv: string;
  /** Prose introducing the command that launches the client. */
  runLabel: string;
  /** Command that launches the client once the env is set. */
  runCommand: string;
};

/**
 * The two providers are structurally identical and differ only in these
 * values, so we keep them as data and render both with the same component.
 */
const PROVIDER_INSTRUCTIONS: Record<LLMFormat, ProviderSpec> = {
  anthropic: {
    clientLabel: 'Claude Code, Claude Agent SDK, or any Anthropic client',
    baseUrlEnv: `export ANTHROPIC_BASE_URL=${localProxyURL}`,
    apiKeyEnv: 'export ANTHROPIC_API_KEY=teleport',
    runLabel: 'Then run Claude Code as usual:',
    runCommand: 'claude',
  },
  openai: {
    clientLabel: 'Codex, or any OpenAI client',
    baseUrlEnv: `export OPENAI_BASE_URL=${localProxyURL}/v1`,
    apiKeyEnv: 'export OPENAI_API_KEY=teleport',
    runLabel: 'Then start Codex:',
    runCommand: 'codex',
  },
};

const apiKeyComment =
  'Any non-empty value works; Teleport swaps in the real key.';

export function LLMAppConnectDialog(props: { app: App; onClose: () => void }) {
  const { app } = props;
  const ctx = useTeleport();
  const { clusterId } = useStickyClusterId();
  const { username, authType } = ctx.storeUser.state;
  const accessRequestId = ctx.storeUser.getAccessRequestId();

  const format: LLMFormat = app.llmFormat === 'openai' ? 'openai' : 'anthropic';

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
        <DialogTitle>Use "{app.name}" inference endpoint</DialogTitle>
      </DialogHeader>

      <DialogContent>
        <Stack gap={4} fullWidth>
          <Step number={1} title="Log in to Teleport">
            <TextSelectCopy
              text={generateTshLoginCommand({
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
            <TextSelectCopy
              text={`tsh proxy app ${app.name} --port ${LOCAL_PROXY_PORT}`}
            />
            <Box>
              This listens on {localProxyURL}. Every request is authenticated
              and audited by Teleport, which also injects the provider API key -
              so no real key is needed locally.
            </Box>
          </Step>

          <Step number={3} title="Point your LLM client at the local proxy">
            <ProviderInstructions spec={PROVIDER_INSTRUCTIONS[format]} />
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
      <TextSelectCopyMulti
        lines={[
          { text: spec.baseUrlEnv },
          { comment: apiKeyComment, text: spec.apiKeyEnv },
        ]}
      />
      <Box>{spec.runLabel}</Box>
      <TextSelectCopy text={spec.runCommand} />
    </>
  );
}
