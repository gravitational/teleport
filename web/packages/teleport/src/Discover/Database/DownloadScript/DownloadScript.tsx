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
import { Box, ButtonSecondary, Label as Pill, Text } from 'design';
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

import {
  ActionButtons,
  DiscoverLabel,
  Header,
  HeaderSubtitle,
  LabelsCreater,
  Mark,
  ResourceKind,
  TextIcon,
  useShowHint,
} from '../../Shared';
import { matchLabels } from '../util';

import type { AgentStepProps } from '../../types';

export default function Container(props: AgentStepProps) {
  const hasDbLabels = props.agentMeta?.agentMatcherLabels?.length;
  const dbLabels = hasDbLabels ? props.agentMeta.agentMatcherLabels : [];
  const [labels, setLabels] = useState<DiscoverLabel[]>(
    // TODO(lisa): we will always be defaulting to asterisks, so
    // instead of defining the default here, define it inside
    // the LabelsCreator (component responsible for rendering
    // label input boxes)
    [{ name: '*', value: '*', isFixed: dbLabels.length === 0 }]
  );

  const [showScript, setShowScript] = useState(false);

  const labelProps = { labels, setLabels, dbLabels };

  return (
    <Validation>
      {({ validator }) => (
        <CatchError
          onRetry={() => clearCachedJoinTokenResult(ResourceKind.Database)}
          fallbackFn={fbProps => (
            <Box>
              <Heading />
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
                <Heading />
                <Labels {...labelProps} disableBtns={true} />
                <ActionButtons onProceed={() => null} disableProceed={true} />
              </Box>
            }
          >
            {!showScript && (
              <LoadedView {...labelProps} setShowScript={setShowScript} />
            )}
            {showScript && (
              <DownloadScript
                {...props}
                {...labelProps}
                validator={validator}
              />
            )}
          </Suspense>
        </CatchError>
      )}
    </Validation>
  );
}

export function DownloadScript(
  props: AgentStepProps & {
    labels: AgentLabel[];
    setLabels(l: AgentLabel[]): void;
    dbLabels: AgentLabel[];
    validator: Validator;
  }
) {
  // Fetches join token.
  const { joinToken } = useJoinTokenSuspender(
    ResourceKind.Database,
    props.labels
  );

  // Starts resource querying interval.
  const { active, result } = usePingTeleport<Database>(
    props.agentMeta.resourceName
  );

  const showHint = useShowHint(active);

  function handleNextStep() {
    props.updateAgentMeta({
      ...props.agentMeta,
      resourceName: result.name,
      db: result,
    });
    props.nextStep();
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
      <Heading />
      <Labels
        labels={props.labels}
        setLabels={props.setLabels}
        disableBtns={true}
        dbLabels={props.dbLabels}
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

const Heading = () => {
  return (
    <>
      <Header>Deploy a Database Service</Header>
      <HeaderSubtitle>
        On the host where you will run the Teleport Database Service, execute
        the generated command that will install and start Teleport with the
        appropriate configuration.
      </HeaderSubtitle>
    </>
  );
};

export const Labels = ({
  labels,
  setLabels,
  disableBtns = false,
  dbLabels,
  showLabelMatchErr = false,
}: {
  labels: AgentLabel[];
  setLabels(l: AgentLabel[]): void;
  disableBtns?: boolean;
  dbLabels: AgentLabel[];
  showLabelMatchErr?: boolean;
}) => {
  const hasDbLabels = dbLabels.length > 0;
  return (
    <Box mb={2}>
      <Text bold>Optionally Define Matcher Labels</Text>
      <Text typography="subtitle1" mb={2}>
        {!hasDbLabels && (
          <>
            Since no labels were defined for the registered database from the
            previous step, the matcher labels are defaulted to wildcards which
            will allow this database service to match any database.
          </>
        )}
        {hasDbLabels && (
          <>
            Default wildcards label allows this database service to match any
            database.
            <br />
            You can define your own labels that this database service will use
            to identify your registered database. The labels you define must
            match the labels that were defined for the registered database (from
            previous step):
          </>
        )}
      </Text>
      {hasDbLabels && (
        <Box mb={3}>
          {dbLabels.map((label, index) => {
            const labelText = `${label.name}: ${label.value}`;
            return (
              <Pill
                key={`${label.name}${label.value}${index}`}
                mr="1"
                kind="secondary"
              >
                {labelText}
              </Pill>
            );
          })}
        </Box>
      )}
      <LabelsCreater
        labels={labels}
        setLabels={setLabels}
        disableBtns={disableBtns || dbLabels.length === 0}
      />
      <Box mt={1} mb={3}>
        {showLabelMatchErr && (
          <TextIcon>
            <Icons.Warning ml={1} color="error.main" />
            The matcher labels must be able to match with the labels defined for
            the registered database. Use wildcards to match with any labels.
          </TextIcon>
        )}
      </Box>
    </Box>
  );
};

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getDbScriptUrl(tokenId)})"`;
}

// hasMatchingLabels will go through each 'agentLabels' and find matches from
// 'dbLabels'. The 'agentLabels' must have same amount of matching labels
// with 'dbLabels' either with asteriks (match all) or by exact match.
//
// `agentLabels` have OR comparison eg:
//  - If agent labels was defined like this [`fruit: apple`, `fruit: banana`]
//    it's translated as `fruit: [apple OR banana]`.
//
// asterisks can be used for keys, values, or both key and value eg:
//  - `fruit: *` match by key `fruit` with any value
//  - `*: apple` match by value `apple` with any key
//  - `*: *` match by any key and any value
export function hasMatchingLabels(
  dbLabels: AgentLabel[],
  agentLabels: AgentLabel[]
) {
  // Convert agentLabels into a map of key of value arrays.
  const matcherLabels: Record<string, string[]> = {};
  agentLabels.forEach(l => {
    if (!matcherLabels[l.name]) {
      matcherLabels[l.name] = [];
    }
    matcherLabels[l.name] = [...matcherLabels[l.name], l.value];
  });

  return matchLabels(dbLabels, matcherLabels);
}

function LoadedView({ labels, setLabels, dbLabels, setShowScript }) {
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
      <Heading />
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
