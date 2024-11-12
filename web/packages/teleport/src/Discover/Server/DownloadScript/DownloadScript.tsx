/**
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

import React, { Suspense, useEffect, useState } from 'react';

import { Box, Indicator, Text, Mark } from 'design';
import * as Icons from 'design/Icon';

import { P } from 'design/Text/Text';

import cfg from 'teleport/config';
import { CatchError } from 'teleport/components/CatchError';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { JoinToken } from 'teleport/services/joinToken';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import { CommandBox } from 'teleport/Discover/Shared/CommandBox';

import {
  HintBox,
  SuccessBox,
  WaitingInfo,
} from 'teleport/Discover/Shared/HintBox';

import { AgentStepProps } from '../../types';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  ResourceKind,
  TextIcon,
} from '../../Shared';

import type { Node } from 'teleport/services/nodes';

const SHOW_HINT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export default function Container(props: AgentStepProps) {
  return (
    <CatchError
      onRetry={() => clearCachedJoinTokenResult([ResourceKind.Server])}
      fallbackFn={fbProps => (
        <Template prevStep={props.prevStep} nextStep={() => null}>
          <TextIcon mt={2} mb={3}>
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            Encountered Error: {fbProps.error.message}
          </TextIcon>
        </Template>
      )}
    >
      <Suspense
        fallback={
          <Box height="144px">
            <Template prevStep={props.prevStep} nextStep={() => null}>
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
  const { joinToken } = useJoinTokenSuspender([ResourceKind.Server]);
  // Starts resource querying interval.
  const { result, active } = usePingTeleport<Node>(joinToken);

  const [showHint, setShowHint] = useState(false);

  useEffect(() => {
    if (active) {
      const id = window.setTimeout(() => setShowHint(true), SHOW_HINT_TIMEOUT);

      return () => window.clearTimeout(id);
    }
  }, [active]);

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

  let hint;
  if (showHint && !result) {
    hint = (
      <HintBox header="We're still looking for your server">
        <Text mb={3}>
          There are a couple of possible reasons for why we haven't been able to
          detect your server.
        </Text>

        <Text mb={1}>
          - The command was not run on the server you were trying to add.
        </Text>

        <Text mb={3}>
          - The Teleport Service could not join this Teleport cluster. Check the
          logs for errors by running <Mark>journalctl -fu teleport</Mark>.
        </Text>

        <Text>
          We'll continue to look for the server whilst you diagnose the issue.
        </Text>
      </HintBox>
    );
  } else if (result) {
    hint = (
      <SuccessBox>Successfully detected your new Teleport instance.</SuccessBox>
    );
  } else {
    hint = (
      <WaitingInfo>
        <TextIcon
          css={`
            white-space: pre;
          `}
        >
          <Icons.Restore size="medium" mr={2} />
        </TextIcon>
        After running the command above, we'll automatically detect your new
        Teleport instance.
      </WaitingInfo>
    );
  }

  return (
    <>
      <Header>Configure Resource</Header>
      <HeaderSubtitle>
        Install and configure the Teleport Service
      </HeaderSubtitle>
      <P mb={4}>Run the following command on the server you want to add.</P>
      <CommandBox>
        <TextSelectCopyMulti
          lines={[{ text: createBashCommand(joinToken.id) }]}
        />
      </CommandBox>
      {hint}
      <ActionButtons
        onProceed={handleNextStep}
        disableProceed={!result}
        onPrev={props.prevStep}
      />
    </>
  );
}

const Template = ({
  nextStep,
  prevStep,
  children,
}: {
  nextStep(): void;
  prevStep(): void;
  children: React.ReactNode;
}) => {
  return (
    <>
      <Header>Configure Resource</Header>
      <HeaderSubtitle>
        Install and configure the Teleport Service.
        <br />
        Run the following command on the server you want to add.
      </HeaderSubtitle>
      <CommandBox>{children}</CommandBox>
      <ActionButtons
        onProceed={nextStep}
        disableProceed={true}
        onPrev={prevStep}
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
};
