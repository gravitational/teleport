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

import { Suspense, useEffect, useState } from 'react';

import { Box, ButtonSecondary, Flex, Mark, Text } from 'design';
import * as Icons from 'design/Icon';
import { H3, Subtitle3 } from 'design/Text/Text';
import Validation, { Validator } from 'shared/components/Validation';

import { CatchError } from 'teleport/components/CatchError';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import {
  HintBox,
  SuccessBox,
  WaitingInfo,
} from 'teleport/Discover/Shared/HintBox';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { ResourceLabelTooltip } from 'teleport/Discover/Shared/ResourceLabelTooltip/ResourceLabelTooltip';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { ResourceLabel } from 'teleport/services/agents';
import { JoinToken } from 'teleport/services/joinToken';
import { Node } from 'teleport/services/nodes';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  LabelsCreater,
  ResourceKind,
  StyledBox,
  TextIcon,
} from '../../Shared';
import { AgentStepProps } from '../../types';

const SHOW_HINT_TIMEOUT = 1000 * 60 * 5; // 5 minutes

export default function Container(props: AgentStepProps) {
  const [labels, setLabels] = useState<ResourceLabel[]>([]);
  const [showScript, setShowScript] = useState(false);

  function toggleShowScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    setShowScript(!showScript);
  }

  const commonProps = {
    labels,
    onChangeLabels: setLabels,
    showScript,
    onShowScript: toggleShowScript,
    onPrev: props.prevStep,
  };

  return (
    <CatchError
      onRetry={() => clearCachedJoinTokenResult([ResourceKind.Server])}
      fallbackFn={fbProps => (
        <>
          <Heading />
          <StepOne
            {...commonProps}
            showScript={false}
            onShowScript={fbProps.retry}
            error={fbProps.error}
          />
        </>
      )}
    >
      <Suspense
        fallback={
          <>
            <Heading />
            <StepOne {...commonProps} processing={true} />
          </>
        }
      >
        <Heading />
        <StepOne {...commonProps} />
        {showScript && <StepTwoWithActionBtns {...props} labels={labels} />}
      </Suspense>
    </CatchError>
  );
}

const Heading = () => (
  <>
    <Header>Configure Resource</Header>
    <HeaderSubtitle>
      Install and configure the Teleport SSH Service
    </HeaderSubtitle>
  </>
);

export function StepOne({
  labels,
  onChangeLabels,
  showScript,
  onShowScript,
  error,
  processing = false,
  onPrev,
}: {
  labels: ResourceLabel[];
  onChangeLabels(l: ResourceLabel[]): void;
  showScript: boolean;
  onShowScript(validator: Validator): void;
  error?: Error;
  processing?: boolean;
  onPrev(): void;
}) {
  const nextLabelTxt = labels.length
    ? 'Finish Adding Labels'
    : 'Skip Adding Labels';
  return (
    <>
      <StyledBox mb={5}>
        <header>
          <H3>Step 1 (Optional)</H3>
          <Flex alignItems="center" gap={1} mb={2}>
            <Subtitle3>Add Labels</Subtitle3>
            <ResourceLabelTooltip resourceKind="server" toolTipPosition="top" />
          </Flex>
        </header>
        <Validation>
          {({ validator }) => (
            <>
              <LabelsCreater
                labels={labels}
                setLabels={onChangeLabels}
                isLabelOptional={true}
                disableBtns={showScript && !error}
                noDuplicateKey={true}
              />
              {error && (
                <TextIcon mt={2} mb={3}>
                  <Icons.Warning
                    size="medium"
                    ml={1}
                    mr={2}
                    color="error.main"
                  />
                  Encountered Error: {error.message}
                </TextIcon>
              )}
              <Box mt={3}>
                <ButtonSecondary
                  width="200px"
                  type="submit"
                  onClick={() => onShowScript(validator)}
                  disabled={processing}
                >
                  {showScript && !error ? 'Edit Labels' : nextLabelTxt}
                </ButtonSecondary>
              </Box>
            </>
          )}
        </Validation>
      </StyledBox>
      {(!showScript || processing || error) && (
        <ActionButtons
          onProceed={() => null}
          disableProceed={true}
          onPrev={onPrev}
        />
      )}
    </>
  );
}

export function StepTwoWithActionBtns(
  props: AgentStepProps & { labels: ResourceLabel[] }
) {
  // Fetches join token.
  const { joinToken } = useJoinTokenSuspender({
    resourceKinds: [ResourceKind.Server],
    suggestedLabels: props.labels,
  });
  // Starts resource querying interval.
  const { result, active } = usePingTeleport<Node>(joinToken);

  const [showHint, setShowHint] = useState(false);

  useEffect(() => {
    if (active) {
      const id = window.setTimeout(() => setShowHint(true), SHOW_HINT_TIMEOUT);

      return () => {
        window.clearTimeout(id);
        clearCachedJoinTokenResult([ResourceKind.Server]);
      };
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
      {joinToken && (
        <>
          <StyledBox mb={5}>
            <header>
              <H3>Step 2</H3>
              <Subtitle3 mb={3}>
                Run the following command on the server you want to add
              </Subtitle3>
            </header>
            <TextSelectCopyMulti
              lines={[{ text: createBashCommand(joinToken.id) }]}
            />
          </StyledBox>
          <Box width="800px">{hint}</Box>
        </>
      )}
      <ActionButtons
        onProceed={handleNextStep}
        disableProceed={!result}
        onPrev={props.prevStep}
      />
    </>
  );
}

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getNodeScriptUrl(tokenId)})"`;
}

export type State = {
  joinToken: JoinToken;
  nextStep(): void;
  regenerateScriptAndRepoll(): void;
};
