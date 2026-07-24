/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import {
  ChangeEvent,
  ChangeEventHandler,
  PropsWithChildren,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonSecondary,
  disappear,
  Flex,
  H1,
  LabelInput,
  Link,
  rotate360,
  Text,
} from 'design';
import { Check, Spinner } from 'design/Icon';
import { LabelContent } from 'design/LabelInput/LabelInput';
import { Gateway } from 'gen-proto-ts/teleport/lib/teleterm/v1/gateway_pb';
import { LoginItem, MenuLogin } from 'shared/components/MenuLogin';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation from 'shared/components/Validation';
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import {
  formatPortRange,
  portRangeSeparator,
} from 'teleterm/services/tshd/app';
import { useAppContext } from 'teleterm/ui/appContextProvider';
import { PortFieldInput } from 'teleterm/ui/components/FieldInputs';
import { useLogger } from 'teleterm/ui/hooks/useLogger';
import { setUpAppGateway } from 'teleterm/ui/services/workspacesService';
import { retryWithRelogin } from 'teleterm/ui/utils';

export function AppGateway(props: {
  gateway: Gateway;
  disconnectAttempt: Attempt<void>;
  changeLocalPort(port: string): void;
  changeLocalPortAttempt: Attempt<void>;
  changeTargetPort(port: string): void;
  changeTargetPortAttempt: Attempt<void>;
  disconnect(): void;
}) {
  const { gateway } = props;
  const ctx = useAppContext();
  const { tshd } = ctx;
  const { targetUri } = gateway;
  const logger = useLogger('AppGateway');

  const {
    changeLocalPort,
    changeLocalPortAttempt,
    changeTargetPort,
    changeTargetPortAttempt,
    disconnectAttempt,
  } = props;
  // It must be possible to update local port while target port is invalid, hence why
  // useDebouncedPortChangeHandler checks the validity of only one input at a time. Otherwise the UI
  // would lose updates to the local port while the target port was invalid.
  const handleLocalPortChange = useDebouncedPortChangeHandler(changeLocalPort);
  const handleTargetPortChange =
    useDebouncedPortChangeHandler(changeTargetPort);

  const isMcp = gateway.protocol === 'MCP';
  const isHttpWebApp = gateway.protocol === 'HTTP';
  const isLLM = gateway.protocol === 'LLM';
  let address = `${gateway.localAddress}:${gateway.localPort}`;
  if (isHttpWebApp || isMcp || isLLM) {
    address = `http://${address}`;
  }

  const llmName = llmDisplayNames[gateway.llmFormat];
  const llmTitle = `${llmName ? `${llmName} ` : ''}Inference Endpoint Connection`;

  // AppGateway doesn't have access to the app resource itself, so it has to decide whether the
  // app is multi-port or not in some other way.
  // For multi-port apps, DocumentGateway comes with targetSubresourceName prefilled to the first
  // port number found in TCP ports. Single-port apps have this field empty.
  // So, if targetSubresourceName is present, then the app must be multi-port. In this case, the
  // user is free to change it and can never provide an empty targetSubresourceName.
  // When the app is not multi-port, targetSubresourceName is empty and the user cannot change it.
  const isMultiPort =
    gateway.protocol === 'TCP' && gateway.targetSubresourceName;
  const targetPortRef = useRef<HTMLInputElement>(null);

  const [tcpPortsAttempt, getTcpPorts] = useAsync(
    useCallback(
      () =>
        retryWithRelogin(ctx, targetUri, () =>
          tshd
            .getApp({ appUri: targetUri })
            .then(({ response }) => response.app.tcpPorts)
        ),
      [ctx, targetUri, tshd]
    )
  );
  const currentTargetPort = parseInt(gateway.targetSubresourceName);
  const getTcpPortsForMenuLogin: () => Promise<LoginItem[]> =
    useCallback(async () => {
      const [tcpPorts, error] = await getTcpPorts();

      if (error) {
        throw error;
      }

      return tcpPorts
        .filter(
          portRange =>
            // Filter out single-port port ranges that are equal to the current port.
            portRange.endPort !== 0 || portRange.port != currentTargetPort
        )
        .map(portRange => ({
          login: formatPortRange(portRange),
          url: '',
        }));
    }, [getTcpPorts, currentTargetPort]);

  const onPortRangeSelect = (_, formattedPortRange: string) => {
    const firstPort = formattedPortRange.split(portRangeSeparator)[0];
    const targetPort = parseInt(firstPort);

    if (Number.isNaN(targetPort)) {
      logger.error('Not a number', firstPort);
      return;
    }

    setUpAppGateway(ctx, targetUri, {
      telemetry: { origin: 'resource_table' },
      targetPort,
      targetProtocol: gateway.protocol,
    });
  };

  return (
    <Flex
      flexDirection="column"
      maxWidth="680px"
      width="100%"
      mx="auto"
      mt="4"
      px="5"
      gap={3}
    >
      <Flex flexDirection="column" gap={2}>
        <Flex justifyContent="space-between" mb="2" flexWrap="wrap" gap={2}>
          <H1>
            {isMcp
              ? 'MCP Server Connection'
              : isLLM
                ? llmTitle
                : 'App Connection'}
          </H1>
          <Flex gap={2}>
            {isMultiPort && (
              <MenuLogin
                getLoginItems={getTcpPortsForMenuLogin}
                onSelect={onPortRangeSelect}
                textTransform="none"
                placeholder="Pick target port"
                ButtonComponent={ButtonSecondary}
                buttonText="Open New Connection"
                anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
                transformOrigin={{ vertical: 'top', horizontal: 'left' }}
              />
            )}
            <ButtonSecondary size="small" onClick={props.disconnect}>
              Close Connection
            </ButtonSecondary>
          </Flex>
        </Flex>

        {disconnectAttempt.status === 'error' && (
          <Alert details={disconnectAttempt.statusText} m={0}>
            Could not close the connection
          </Alert>
        )}

        <Flex as="form" gap={2}>
          <Validation>
            <LabelWithAttemptStatus
              text="Local Port"
              attempt={changeLocalPortAttempt}
            >
              <PortFieldInput
                defaultValue={gateway.localPort}
                onChange={handleLocalPortChange}
                mb={0}
              />
            </LabelWithAttemptStatus>
            {isMultiPort && (
              <LabelWithAttemptStatus
                text="Target Port"
                attempt={changeTargetPortAttempt}
                required
              >
                <PortFieldInput
                  required
                  defaultValue={gateway.targetSubresourceName}
                  onChange={handleTargetPortChange}
                  mb={0}
                  ref={targetPortRef}
                />
              </LabelWithAttemptStatus>
            )}
          </Validation>
        </Flex>
      </Flex>

      <Flex flexDirection="column" gap={2}>
        {isLLM ? (
          <LlmInstructions
            llmFormat={gateway.llmFormat}
            llmProvider={gateway.llmProvider}
            address={address}
          />
        ) : (
          <Box>
            <Text>
              {isMcp
                ? 'Access the MCP server with a streamable-HTTP-compatible client like "mcp-remote" at:'
                : 'Access the app at:'}
            </Text>
            <TextSelectCopy mt={1} text={address} bash={false} />
          </Box>
        )}

        {changeLocalPortAttempt.status === 'error' && (
          <Alert details={changeLocalPortAttempt.statusText} m={0}>
            Could not change the local port
          </Alert>
        )}

        {changeTargetPortAttempt.status === 'error' && (
          <Alert details={changeTargetPortAttempt.statusText} m={0}>
            Could not change the target port
          </Alert>
        )}

        {tcpPortsAttempt.status === 'error' && (
          <Alert kind="warning" details={tcpPortsAttempt.statusText} m={0}>
            Could not fetch available target ports
          </Alert>
        )}

        <Text>
          The connection is made through an authenticated proxy so no extra
          credentials are necessary. See{' '}
          <Link
            href="https://goteleport.com/docs/connect-your-client/teleport-connect/#creating-an-authenticated-tunnel"
            target="_blank"
          >
            the documentation
          </Link>{' '}
          for more details.
        </Text>
      </Flex>
    </Flex>
  );
}

const apiKeyComment =
  'Any non-empty value works; Teleport swaps in the real key.';

const llmDisplayNames: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
};

type LlmEnvLine = { text: string; comment?: string };

type LlmSpec = {
  /** Client(s) named in the instructions, e.g. "Anthropic client (ex. …)". */
  clientLabel: string;
  /**
   * `export <variable>=<value>` lines to set before launching the client.
   */
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
 */
function getLlmSpec(
  llmFormat: string,
  llmProvider: string,
  address: string
): LlmSpec | undefined {
  if (llmFormat === 'openai') {
    // A Bedrock-backed endpoint needs Codex's amazon-bedrock model provider:
    // its plain OpenAI provider sends a payload Bedrock rejects. That provider
    // config also carries the address and (placeholder) auth, so no environment
    // variables are needed. Requires Codex 0.145.0+.
    if (llmProvider === 'bedrock') {
      return {
        clientLabel: 'OpenAI client (ex. Codex)',
        runNote:
          'This endpoint is served by Amazon Bedrock, so Codex must use its Bedrock model provider (requires Codex 0.145.0+):',
        runCommand:
          `codex -c model_providers.amazon-bedrock.base_url=${address} ` +
          `-c model_providers.amazon-bedrock.auth.command=cat ` +
          `-c model_provider=amazon-bedrock`,
      };
    }
    return {
      clientLabel: 'OpenAI client (ex. Codex)',
      envLines: [
        { text: `export OPENAI_BASE_URL=${address}/v1` },
        { text: 'export OPENAI_API_KEY=teleport', comment: apiKeyComment },
      ],
      runNote:
        'Codex ignores the base-URL variable, so pass the address inline:',
      runCommand: `codex -c openai_base_url=${address}/v1`,
    };
  }

  if (llmFormat === 'anthropic') {
    const envLines: LlmEnvLine[] = [
      { text: `export ANTHROPIC_BASE_URL=${address}` },
      { text: 'export ANTHROPIC_API_KEY=teleport', comment: apiKeyComment },
    ];
    if (llmProvider === 'bedrock') {
      envLines.push({
        text: 'export CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1',
        comment: 'Required when the endpoint is served by Amazon Bedrock.',
      });
    }
    return {
      clientLabel: 'Anthropic client (ex. Claude Code, Claude Agent SDK)',
      envLines,
      runCommand: 'claude',
    };
  }

  return undefined;
}

/**
 * LlmInstructions tells the user how to point their LLM client at the running
 * local proxy. Teleport authenticates and audits every request and injects the
 * provider API key, so no real key is needed locally.
 */
function LlmInstructions(props: {
  llmFormat: string;
  llmProvider: string;
  address: string;
}) {
  const spec = getLlmSpec(props.llmFormat, props.llmProvider, props.address);

  if (!spec) {
    return (
      <Box>
        <Text>Point your LLM client at the local proxy:</Text>
        <TextSelectCopy mt={1} text={props.address} bash={false} />
      </Box>
    );
  }

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

const LabelWithAttemptStatus = (
  props: PropsWithChildren<{
    text: string;
    attempt: Attempt<unknown>;
    required?: boolean;
  }>
) => (
  <LabelInput
    mb={0}
    css={`
      width: fit-content;
    `}
  >
    <Flex
      alignItems="center"
      justifyContent="space-between"
      mb={1}
      css={`
        position: relative;
      `}
    >
      <LabelContent required={props.required}>{props.text}</LabelContent>
      {props.attempt.status === 'processing' && (
        <AnimatedSpinner color="interactive.tonal.neutral.2" size="small" />
      )}
      {props.attempt.status === 'success' && (
        // CSS animations are repeated whenever the parent goes from `display: none` to something
        // else. As a result, we need to unmount the animated check so that the animation is not
        // repeated when the user switches to this tab.
        // https://www.w3.org/TR/css-animations-1/#example-4e34d7ba
        <UnmountAfter
          timeoutMs={disappearanceDelayMs + disappearanceDurationMs}
        >
          <DisappearingCheck
            color="success.main"
            size="small"
            title={`${props.text} successfully updated`}
          />
        </UnmountAfter>
      )}
    </Flex>
    {props.children}
  </LabelInput>
);

/**
 * useDebouncedPortChangeHandler returns a debounced change handler that calls the change function
 * only if the input from which the event originated is valid.
 */
const useDebouncedPortChangeHandler = (
  changeFunc: (port: string) => void
): ChangeEventHandler<HTMLInputElement> =>
  useMemo(
    () =>
      debounce((event: ChangeEvent<HTMLInputElement>) => {
        if (event.target.reportValidity()) {
          changeFunc(event.target.value);
        }
      }, 1000),
    [changeFunc]
  );

const AnimatedSpinner = styled(Spinner)`
  animation: ${rotate360} 1.5s infinite linear;
  // The spinner needs to be positioned absolutely so that the fact that it's spinning
  // doesn't affect the size of the parent.
  position: absolute;
  right: 0;
  top: 0;
`;

const disappearanceDelayMs = 1000;
const disappearanceDurationMs = 200;

const DisappearingCheck = styled(Check)`
  opacity: 1;
  animation: ${disappear};
  animation-delay: ${disappearanceDelayMs}ms;
  animation-duration: ${disappearanceDurationMs}ms;
  animation-fill-mode: forwards;
`;

const UnmountAfter = ({
  timeoutMs,
  children,
}: PropsWithChildren<{ timeoutMs: number }>) => {
  const [isMounted, setIsMounted] = useState(true);

  useEffect(() => {
    const timeout = setTimeout(() => {
      setIsMounted(false);
    }, timeoutMs);

    return () => {
      clearTimeout(timeout);
    };
  }, [timeoutMs]);

  return isMounted ? children : null;
};
