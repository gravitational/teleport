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

import { Fragment, ReactNode } from 'react';

import { ButtonSecondary, Stack, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import { generateTshLoginCommand } from 'teleport/lib/util';
import { App, LLMFormat, LLMProvider } from 'teleport/services/apps';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

/**
 * Port the instructions pin the local proxy to, so later steps can reference a
 * concrete address instead of a randomized one.
 */
const LOCAL_PROXY_PORT = '3000';
const localProxyURL = `http://127.0.0.1:${LOCAL_PROXY_PORT}`;

const apiKeyComment =
  'Any non-empty value works; Teleport swaps in the real key.';

type EnvLine = { text: string; comment?: string };

type ProviderSpec = {
  /** Provider name shown in the dialog title, e.g. "Anthropic". */
  name: string;
  /**
   * Client(s) named in the "Point your … client at the local proxy" step, e.g.
   * "Anthropic client (ex. Claude Code, Claude Agent SDK)".
   */
  clientLabel: string;
  /**
   * `export <variable>=<value>` lines to set before launching the client.
   * Omitted for clients that take all their configuration on the command line
   * instead (e.g. Codex against a Bedrock-backed endpoint).
   */
  envLines?: EnvLine[];
  /** Title of the final step, e.g. "Run Claude Code." or "Start Codex." */
  runTitle: string;
  /** Optional note shown under the final step's title. */
  runNote?: string;
  /** Command that launches (and, for Codex, configures) the client. */
  runCommand: string;
};

/**
 * getProviderSpec returns the client-specific instructions for an inference
 * endpoint. They depend on the API format and the provider eg. Codex ignores
 * base-URL environment variables so it needs the address inline, and the
 * inference endpoint served by Bedrock needs an extra environment variable to
 * disable beta features.
 */
function getProviderSpec(
  format: LLMFormat,
  provider: LLMProvider | undefined
): ProviderSpec {
  if (format === 'openai') {
    // A Bedrock-backed endpoint needs Codex's amazon-bedrock model provider:
    // its plain OpenAI provider sends a payload Bedrock rejects. That provider
    // config also carries the address and (placeholder) auth, so no environment
    // variables are needed. Requires Codex 0.145.0+.
    if (provider === 'bedrock') {
      return {
        name: 'OpenAI',
        clientLabel: 'OpenAI client',
        runTitle: 'Start Codex.',
        runNote:
          'This endpoint is served by Amazon Bedrock, so Codex must use its Bedrock model provider (requires Codex 0.145.0+):',
        runCommand:
          `codex -c model_providers.amazon-bedrock.base_url=${localProxyURL} ` +
          `-c model_providers.amazon-bedrock.auth.command=cat ` +
          `-c model_provider=amazon-bedrock`,
      };
    }
    return {
      name: 'OpenAI',
      clientLabel: 'OpenAI client',
      envLines: [
        { text: `export OPENAI_BASE_URL=${localProxyURL}/v1` },
        { text: 'export OPENAI_API_KEY=teleport', comment: apiKeyComment },
      ],
      runTitle: 'Start Codex.',
      runNote:
        'Codex ignores the base-URL variable, so pass the address inline:',
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
    name: 'Anthropic',
    clientLabel: 'Anthropic client (ex. Claude Code, Claude Agent SDK)',
    envLines,
    runTitle: 'Run Claude Code.',
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

  let stepNumber = 1;

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
        <DialogTitle>Using your {spec.name} inference resource</DialogTitle>
      </DialogHeader>

      <DialogContent>
        <Stack gap={4} fullWidth>
          <Step number={stepNumber++} title="Log into Teleport.">
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
            number={stepNumber++}
            title="Start a local proxy for the inference endpoint."
          >
            <TextSelectCopy
              text={`tsh proxy app ${app.name} --port ${LOCAL_PROXY_PORT}`}
            />
            <Text color="text.slightlyMuted">
              This listens on {localProxyURL}. Every request is authenticated
              and audited by Teleport, which also injects the provider API key -
              so no real key is needed locally.
            </Text>
          </Step>

          {spec.envLines && (
            <Step
              number={stepNumber++}
              title={`Point your ${spec.clientLabel} at the local proxy.`}
            >
              {spec.envLines.map((line, index) => (
                <Fragment key={index}>
                  {line.comment && (
                    <Text color="text.slightlyMuted">{line.comment}</Text>
                  )}
                  <TextSelectCopy text={line.text} />
                </Fragment>
              ))}
            </Step>
          )}

          <Step
            number={stepNumber++}
            title={spec.runTitle}
            subtitle={spec.runNote}
          >
            <TextSelectCopy text={spec.runCommand} />
          </Step>
        </Stack>
      </DialogContent>

      <DialogFooter>
        <ButtonSecondary onClick={props.onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

function Step(props: {
  number: number;
  title: string;
  subtitle?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Stack fullWidth gap={2}>
      <Text bold>
        {props.number}. {props.title}
      </Text>
      {props.subtitle && (
        <Text color="text.slightlyMuted">{props.subtitle}</Text>
      )}
      {props.children}
    </Stack>
  );
}
