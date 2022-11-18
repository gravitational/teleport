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

import React, { useState, Suspense } from 'react';
import {
  Text,
  Box,
  ButtonSecondary,
  Flex,
  ButtonIcon,
  ButtonText,
} from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, {
  useValidation,
  Validator,
} from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { CatchError } from 'teleport/components/CatchError';
import {
  useJoinToken,
  clearCachedJoinTokenResult,
} from 'teleport/Discover/Shared/JoinTokenContext';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { CommandWithTimer } from 'teleport/Discover/Shared/CommandWithTimer';
import { AgentLabel } from 'teleport/services/agents';
import cfg from 'teleport/config';
import { Database } from 'teleport/services/databases';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  ResourceKind,
  TextIcon,
} from '../../Shared';

import type { AgentStepProps } from '../../types';
import type { Poll } from 'teleport/Discover/Shared/CommandWithTimer';

export default function Container(
  props: AgentStepProps & { runJoinTokenPromise?: boolean }
) {
  const [showScript, setShowScript] = useState(props.runJoinTokenPromise);
  const [labels, setLabels] = useState<AgentLabel[]>([
    { name: '*', value: '*' },
  ]);

  function handleGenerateCommand(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    setShowScript(true);
  }

  return (
    <Validation>
      {({ validator }) => (
        <CatchError
          onRetry={clearCachedJoinTokenResult}
          fallbackFn={props => (
            <Box>
              <Heading />
              <Labels labels={labels} setLabels={setLabels} />
              <ButtonSecondary width="200px" onClick={props.retry}>
                Generate Command
              </ButtonSecondary>
              <Box>
                <TextIcon mt={3}>
                  <Icons.Warning ml={1} color="danger" />
                  Encountered Error: {props.error.message}
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
                <Labels
                  labels={labels}
                  setLabels={setLabels}
                  disableAddBtn={true}
                />
                <ButtonSecondary width="200px" disabled={true}>
                  Generate Command
                </ButtonSecondary>
                <ActionButtons onProceed={() => null} disableProceed={true} />
              </Box>
            }
          >
            {!showScript && (
              <Box>
                <Heading />
                <Labels labels={labels} setLabels={setLabels} />
                <ButtonSecondary
                  width="200px"
                  onClick={() => handleGenerateCommand(validator)}
                >
                  Generate Command
                </ButtonSecondary>
                <ActionButtons onProceed={() => null} disableProceed={true} />
              </Box>
            )}
            {showScript && (
              <DownloadScript
                {...props}
                labels={labels}
                setLabels={setLabels}
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
  }
) {
  // Fetches join token.
  const { joinToken, reloadJoinToken, timeout } = useJoinToken(
    ResourceKind.Database,
    props.labels
  );

  // Starts resource querying interval.
  const {
    timedOut: pollingTimedOut,
    start,
    result,
  } = usePingTeleport<Database>();

  function regenerateScriptAndRepoll() {
    reloadJoinToken();
    start();
  }

  function handleNextStep() {
    props.updateAgentMeta({
      ...props.agentMeta,
      resourceName: result.name,
      db: result,
    });
  }

  let poll: Poll = { state: 'polling' };
  if (pollingTimedOut) {
    poll = {
      state: 'error',
      error: {
        reasonContents: [
          <>
            The command was not run on the server you were trying to add,
            regenerate command and try again.
          </>,
          // TODO (lisa): not sure what this message should be.
          // <>
          //   The Teleport Service could not join this Teleport cluster. Check the
          //   logs for errors by running <br />
          //   <Mark>kubectl logs -l app=teleport-agent -n {namespace}</Mark>
          // </>,
        ],
      },
    };
  } else if (result) {
    poll = { state: 'success' };
  }

  return (
    <Box>
      <Heading />
      <Labels
        labels={props.labels}
        setLabels={props.setLabels}
        disableAddBtn={poll.state === 'polling'}
      />
      <ButtonSecondary
        width="200px"
        disabled={poll.state === 'polling'}
        onClick={regenerateScriptAndRepoll}
        mb={3}
      >
        Regenerate Command
      </ButtonSecondary>
      <CommandWithTimer
        command={createBashCommand(joinToken.id)}
        poll={poll}
        pollingTimeout={timeout}
      />
      <ActionButtons
        onProceed={handleNextStep}
        disableProceed={poll.state !== 'success' || props.labels.length === 0}
      />
    </Box>
  );
}

const Heading = () => {
  return (
    <>
      <Header>Deploy a Database Service</Header>
      <HeaderSubtitle>TODO lorem ipsum dolores</HeaderSubtitle>
    </>
  );
};

export const Labels = ({
  labels,
  setLabels,
  disableAddBtn = false,
}: {
  labels: AgentLabel[];
  setLabels(l: AgentLabel[]): void;
  disableAddBtn?: boolean;
}) => {
  const validator = useValidation() as Validator;

  function addLabel() {
    // Prevent adding more rows if there are
    // empty input fields. After checking,
    // reset the validator so the newly
    // added empty input boxes are not
    // considered an error.
    if (!validator.validate()) {
      return;
    }
    validator.reset();
    setLabels([...labels, { name: '', value: '' }]);
  }

  function removeLabel(index: number) {
    if (labels.length === 1) {
      // Since at least one label is required
      // instead of removing the row, clear
      // the input and turn on error.
      const newList = [...labels];
      newList[index] = { name: '', value: '' };
      setLabels(newList);

      validator.validate();
      return;
    }
    const newList = [...labels];
    newList.splice(index, 1);
    setLabels(newList);
  }

  const handleChange = (
    event: React.ChangeEvent<HTMLInputElement>,
    index: number,
    labelField: keyof AgentLabel
  ) => {
    const { value } = event.target;
    const newList = [...labels];
    newList[index] = { ...newList[index], [labelField]: value };
    setLabels(newList);
  };

  return (
    <Box mb={2}>
      <Text bold>Labels</Text>
      <Text typography="subtitle1" mb={3}>
        At least one label is required to help this service identify your
        database.
      </Text>
      <Flex mt={2}>
        <Box width="170px" mr="3">
          Key{' '}
          <span css={{ fontSize: '12px', fontWeight: 'lighter' }}>
            (required field)
          </span>
        </Box>
        <Box>
          Value{' '}
          <span css={{ fontSize: '12px', fontWeight: 'lighter' }}>
            (required field)
          </span>
        </Box>
      </Flex>
      <Box>
        {labels.map((label, index) => {
          return (
            <Box mb={2} key={index}>
              <Flex alignItems="center">
                <FieldInput
                  Input
                  rule={requiredField('required')}
                  autoFocus
                  value={label.name}
                  placeholder="label key"
                  width="170px"
                  mr={3}
                  mb={0}
                  onChange={e => handleChange(e, index, 'name')}
                />
                <FieldInput
                  rule={requiredField('required')}
                  value={label.value}
                  placeholder="label value"
                  width="170px"
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                />
                <ButtonIcon
                  size={1}
                  title="Remove Label"
                  onClick={() => removeLabel(index)}
                >
                  <Icons.Trash />
                </ButtonIcon>
              </Flex>
            </Box>
          );
        })}
      </Box>
      <ButtonText
        onClick={addLabel}
        css={`
          padding-left: 0px;
          &:disabled {
            .icon-add {
              opacity: 0.35;
            }
            pointer-events: none;
          }
        `}
        disabled={disableAddBtn}
      >
        <Icons.Add
          className="icon-add"
          disabled={disableAddBtn}
          css={`
            font-weight: bold;
            letter-spacing: 4px;
            margin-top: -2px;
            &:after {
              content: ' ';
            }
          `}
        />
        Add New Label
      </ButtonText>
    </Box>
  );
};

function createBashCommand(tokenId: string) {
  return `sudo bash -c "$(curl -fsSL ${cfg.getDbScriptUrl(tokenId)})"`;
}
