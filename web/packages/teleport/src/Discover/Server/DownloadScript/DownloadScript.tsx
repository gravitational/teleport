/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { Suspense } from 'react';
import { Box, Indicator } from 'design';
import * as Icons from 'design/Icon';

import cfg from 'teleport/config';
import { CatchError } from 'teleport/components/CatchError';
import {
  useJoinToken,
  clearCachedJoinTokenResult,
} from 'teleport/Discover/Shared/JoinTokenContext';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { JoinToken } from 'teleport/services/joinToken';
import { CommandWithTimer } from 'teleport/Discover/Shared/CommandWithTimer';
import {
  PollBox,
  PollState,
} from 'teleport/Discover/Shared/CommandWithTimer/CommandWithTimer';

import { AgentStepProps } from '../../types';
import {
  ActionButtons,
  ButtonBlueText,
  Header,
  HeaderSubtitle,
  Mark,
  ResourceKind,
  TextIcon,
} from '../../Shared';

import type { Node } from 'teleport/services/nodes';
import type { Poll } from 'teleport/Discover/Shared/CommandWithTimer';

export default function Container(props: AgentStepProps) {
  return (
    <CatchError
      onRetry={clearCachedJoinTokenResult}
      fallbackFn={props => (
        <Template pollState="error" nextStep={() => null}>
          <TextIcon mt={2} mb={3}>
            <Icons.Warning ml={1} color="danger" />
            Encountered Error: {props.error.message}
          </TextIcon>
          <ButtonBlueText ml={2} onClick={props.retry}>
            Refetch a command
          </ButtonBlueText>
        </Template>
      )}
    >
      <Suspense
        fallback={
          <Box height="144px">
            <Template nextStep={() => null}>
              <Box textAlign="center" height="108px">
                <Indicator delay="none" />
              </Box>
            </Template>
          </Box>
        }
      >
        <DownloadScript {...props} />
      </Suspense>
    </CatchError>
  );
}

export function DownloadScript(props: AgentStepProps) {
  // Fetches join token.
  const { joinToken, reloadJoinToken, timeout } = useJoinToken(
    ResourceKind.Server
  );
  // Starts resource querying interval.
  const { timedOut: pollingTimedOut, start, result } = usePingTeleport<Node>();

  function regenerateScriptAndRepoll() {
    reloadJoinToken();
    start();
  }

  let poll: Poll = {
    state: 'polling',
    customStateDesc: 'Waiting for Teleport SSH Service',
  };
  if (pollingTimedOut) {
    poll = {
      state: 'error',
      error: {
        customErrContent: (
          <>
            We could not detect the server you were trying to add.{' '}
            <ButtonBlueText ml={1} onClick={regenerateScriptAndRepoll}>
              Generate a new command
            </ButtonBlueText>
          </>
        ),
        reasonContents: [
          <>The command was not run on the server you were trying to add</>,
          <>
            The Teleport SSH Service could not join this Teleport cluster. Check
            the logs for errors by running <br />
            <Mark>journalctl status teleport</Mark>
          </>,
        ],
      },
    };
  } else if (result) {
    poll = {
      state: 'success',
      customStateDesc: 'The server successfully joined this Teleport cluster',
    };
  }

  function handleNextStep() {
    props.updateAgentMeta({
      ...props.agentMeta,
      // Node is an oddity in that the hostname is the more
      // user identifiable resource name and what user expects
      // as the resource name.
      resourceName: result.hostname,
      node: result,
    });
    props.nextStep();
  }

  return (
    <>
      <Header>Configure Resource</Header>
      <HeaderSubtitle>
        Install and configure the Teleport SSH Service.
        <br />
        Run the following command on the server you want to add.
      </HeaderSubtitle>
      <CommandWithTimer
        command={createBashCommand(joinToken.id)}
        poll={poll}
        pollingTimeout={timeout}
      />
      <ActionButtons
        onProceed={handleNextStep}
        disableProceed={poll.state !== 'success'}
      />
    </>
  );
}

const Template = ({
  nextStep,
  pollState,
  children,
}: {
  nextStep(): void;
  pollState?: PollState;
  children: React.ReactNode;
}) => {
  return (
    <>
      <Header>Configure Resource</Header>
      <HeaderSubtitle>
        Install and configure the Teleport SSH Service.
        <br />
        Run the following command on the server you want to add.
      </HeaderSubtitle>
      <PollBox pollState={pollState}>{children}</PollBox>
      <ActionButtons
        onProceed={nextStep}
        disableProceed={
          !pollState || pollState === 'error' || pollState === 'polling'
        }
      />
    </>
  );
};

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(tokenId)})"`;
}

export type State = {
  joinToken: JoinToken;
  nextStep(): void;
  regenerateScriptAndRepoll(): void;
  poll: Poll;
  pollTimeout: number;
};
