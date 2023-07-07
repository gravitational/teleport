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

import React, { Suspense, useState, useEffect } from 'react';
import { Box, ButtonSecondary, Text } from 'design';
import * as Icons from 'design/Icon';
import Validation, { Validator } from 'shared/components/Validation';

import { CatchError } from 'teleport/components/CatchError';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { AgentLabel } from 'teleport/services/agents';
import cfg from 'teleport/config';
import { Database } from 'teleport/services/databases';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import {
  HintBox,
  SuccessBox,
  WaitingInfo,
} from 'teleport/Discover/Shared/HintBox';

import { CommandBox } from 'teleport/Discover/Shared/CommandBox';
import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { DatabaseLocation } from 'teleport/Discover/SelectResource';
import { DiscoverEventStatus } from 'teleport/services/userEvent';

import {
  ActionButtons,
  AlternateInstructionButton,
  DiscoverLabel,
  Header,
  HeaderSubtitle,
  Mark,
  ResourceKind,
  TextIcon,
  useShowHint,
} from '../../../Shared';
import { Labels, hasMatchingLabels } from '../../common';
import { DeployServiceProp } from '../DeployService';

export default function Container({ toggleDeployMethod }: DeployServiceProp) {
  const { agentMeta } = useDiscover();
  const hasDbLabels = agentMeta?.agentMatcherLabels?.length;
  const dbLabels = hasDbLabels ? agentMeta.agentMatcherLabels : [];
  const [labels, setLabels] = useState<DiscoverLabel[]>(
    // TODO(lisa): we will always be defaulting to asterisks, so
    // instead of defining the default here, define it inside
    // the LabelsCreator (component responsible for rendering
    // label input boxes)
    [{ name: '*', value: '*', isFixed: dbLabels.length === 0 }]
  );

  const [showScript, setShowScript] = useState(false);

  const labelProps = { labels, setLabels, dbLabels };

  const heading = <Heading toggleDeployMethod={toggleDeployMethod} />;

  return (
    <Validation>
      {({ validator }) => (
        <CatchError
          onRetry={() => clearCachedJoinTokenResult(ResourceKind.Database)}
          fallbackFn={fbProps => (
            <Box>
              {heading}
              <Labels {...labelProps} />
              <Box>
                <TextIcon mt={3}>
                  <Icons.Warning ml={1} color="error.main" />
                  Encountered Error: {fbProps.error.message}
                </TextIcon>
              </Box>
              <ActionButtons onProceed={() => null} disableProceed={true} />
            </Box>
          )}
        >
          <Suspense
            fallback={
              <Box>
                {heading}
                <Labels {...labelProps} disableBtns={true} />
                <ActionButtons onProceed={() => null} disableProceed={true} />
              </Box>
            }
          >
            {!showScript && (
              <LoadedView
                {...labelProps}
                setShowScript={setShowScript}
                toggleDeployMethod={toggleDeployMethod}
              />
            )}
            {showScript && (
              <ManualDeploy
                {...labelProps}
                validator={validator}
                toggleDeployMethod={toggleDeployMethod}
              />
            )}
          </Suspense>
        </CatchError>
      )}
    </Validation>
  );
}

export function ManualDeploy(props: {
  labels: AgentLabel[];
  setLabels(l: AgentLabel[]): void;
  dbLabels: AgentLabel[];
  validator: Validator;
  toggleDeployMethod(): void;
}) {
  const { agentMeta, updateAgentMeta, nextStep, emitEvent } = useDiscover();

  // Fetches join token.
  const { joinToken } = useJoinTokenSuspender(
    ResourceKind.Database,
    props.labels
  );

  // Starts resource querying interval.
  const { active, result } = usePingTeleport<Database>(agentMeta.resourceName);

  const showHint = useShowHint(active);

  function handleNextStep() {
    updateAgentMeta({
      ...agentMeta,
      resourceName: result.name,
      db: result,
      serviceDeployedMethod: 'manual',
    });
    nextStep();

    emitEvent(
      { stepStatus: DiscoverEventStatus.Success }
      // TODO(lisa) uncomment after backend handles this field
      // { deployMethod: 'manual' }
    );
  }

  let hint;
  if (showHint && !result) {
    hint = (
      <HintBox header="We're still looking for your database service">
        <Text mb={3}>
          There are a couple of possible reasons for why we haven't been able to
          detect your database service.
        </Text>

        <Text mb={1}>
          - The command was not run on the server you were trying to add.
        </Text>

        <Text mb={3}>
          - The Teleport Database Service could not join this Teleport cluster.
          Check the logs for errors by running{' '}
          <Mark>journalctl -fu teleport</Mark>.
        </Text>

        <Text>
          We'll continue to look for the database service whilst you diagnose
          the issue.
        </Text>
      </HintBox>
    );
  } else if (result) {
    hint = (
      <SuccessBox>
        Successfully detected your new Teleport database service.
      </SuccessBox>
    );
  } else {
    hint = (
      <WaitingInfo>
        <TextIcon
          css={`
            white-space: pre;
          `}
        >
          <Icons.Restore fontSize={4} />
        </TextIcon>
        After running the command above, we'll automatically detect your new
        Teleport database service.
      </WaitingInfo>
    );
  }

  return (
    <Box>
      <Heading toggleDeployMethod={props.toggleDeployMethod} />
      <Labels
        labels={props.labels}
        setLabels={props.setLabels}
        disableBtns={true}
        dbLabels={props.dbLabels}
        region={(agentMeta as DbMeta).selectedAwsRdsDb?.region}
      />
      <Box mt={6}>
        <CommandBox>
          <TextSelectCopyMulti
            lines={[{ text: createBashCommand(joinToken.id) }]}
          />
        </CommandBox>
        {hint}
      </Box>
      <ActionButtons
        onProceed={handleNextStep}
        disableProceed={!result || props.labels.length === 0}
      />
    </Box>
  );
}

const Heading = ({ toggleDeployMethod }: { toggleDeployMethod(): void }) => {
  const { resourceSpec } = useDiscover();
  const isAwsRds = resourceSpec.dbMeta?.location === DatabaseLocation.Aws;
  const canChangeDeployMethod = isAwsRds && toggleDeployMethod;

  return (
    <>
      <Header>Manually Deploy a Database Service</Header>
      <HeaderSubtitle>
        On the host where you will run the Teleport Database Service, execute
        the generated command that will install and start Teleport with the
        appropriate configuration.
        {canChangeDeployMethod && (
          <>
            <br /> Want us to deploy the database service for you?{' '}
            <AlternateInstructionButton onClick={toggleDeployMethod} />
          </>
        )}
      </HeaderSubtitle>
    </>
  );
};

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getDbScriptUrl(tokenId)})"`;
}

function LoadedView({
  labels,
  setLabels,
  dbLabels,
  setShowScript,
  toggleDeployMethod,
}) {
  const [showLabelMatchErr, setShowLabelMatchErr] = useState(true);

  useEffect(() => {
    // Turn off error once user changes labels.
    if (showLabelMatchErr) {
      setShowLabelMatchErr(false);
    }
  }, [labels]);

  function handleGenerateCommand() {
    if (!hasMatchingLabels(dbLabels, labels)) {
      setShowLabelMatchErr(true);
      return;
    }

    setShowLabelMatchErr(false);
    setShowScript(true);
  }

  return (
    <Box>
      <Heading toggleDeployMethod={toggleDeployMethod} />
      <Labels
        labels={labels}
        setLabels={setLabels}
        dbLabels={dbLabels}
        showLabelMatchErr={showLabelMatchErr}
      />
      <ButtonSecondary width="200px" onClick={handleGenerateCommand}>
        Generate Command
      </ButtonSecondary>
      <ActionButtons onProceed={() => null} disableProceed={true} />
    </Box>
  );
}
