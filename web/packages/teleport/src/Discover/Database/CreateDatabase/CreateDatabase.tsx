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

import React, { useState } from 'react';
import {
  Text,
  Box,
  Flex,
  AnimatedProgressBar,
  ButtonPrimary,
  ButtonSecondary,
} from 'design';
import Dialog, { DialogContent } from 'design/DialogConfirmation';
import * as Icons from 'design/Icon';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import TextEditor from 'shared/components/TextEditor';

import { Timeout } from 'teleport/Discover/Shared/Timeout';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  LabelsCreater,
  Mark,
  TextIcon,
} from '../../Shared';
import { dbCU } from '../../yamlTemplates';
import { getDatabaseProtocol } from '../resources';

import { useCreateDatabase, State } from './useCreateDatabase';

import type { AgentStepProps } from '../../types';
import type { AgentLabel } from 'teleport/services/agents';
import type { Attempt } from 'shared/hooks/useAttemptNext';

export function CreateDatabase(props: AgentStepProps) {
  const state = useCreateDatabase(props);
  return <CreateDatabaseView {...state} />;
}

export function CreateDatabaseView({
  attempt,
  clearAttempt,
  registerDatabase,
  canCreateDatabase,
  engine,
  pollTimeout,
}: State) {
  const [dbName, setDbName] = useState('');
  const [dbUri, setDbUri] = useState('');
  const [labels, setLabels] = useState<AgentLabel[]>([]);

  // TODO(lisa): default ports depend on type of database.
  const [dbPort, setDbPort] = useState('5432');

  // TODO (lisa or ryan): these depend on if user chose AWS options:
  // const [awsAccountId, setAwsAccountId] = useState('')
  // const [awsResourceId, setAwsResourceId] = useState('')

  function handleOnProceed(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    // TODO (lisa or ryan): preserve "self hosted" or "aws"
    // and protocol on first step, and use it here.
    registerDatabase({
      labels,
      name: dbName,
      uri: `${dbUri}:${dbPort}`,
      protocol: getDatabaseProtocol(engine),
      // TODO (lisa or ryan) add AWS fields
    });
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box maxWidth="800px">
          <Header>Register a Database</Header>
          <HeaderSubtitle>
            Create a new database resource for the database server.
          </HeaderSubtitle>
          {!canCreateDatabase && (
            <Box>
              <Text>
                You don't have permission to register a database.
                <br />
                Please ask your Teleport administrator to update your role and
                add the <Mark>db</Mark> rule:
              </Text>
              <Flex minHeight="195px" mt={3}>
                <TextEditor
                  readOnly={true}
                  data={[{ content: dbCU, type: 'yaml' }]}
                />
              </Flex>
            </Box>
          )}
          {canCreateDatabase && (
            <>
              <Box width="500px" mb={2}>
                <FieldInput
                  label="Database Name"
                  rule={requiredField('database name is required')}
                  autoFocus
                  value={dbName}
                  placeholder="Enter database name"
                  onChange={e => setDbName(e.target.value)}
                />
              </Box>
              <Box width="500px" mb={2}>
                <FieldInput
                  label="Database Connection Endpoint"
                  rule={requiredField(
                    'database connection endpoint is required'
                  )}
                  value={dbUri}
                  placeholder="db.example.com"
                  onChange={e => setDbUri(e.target.value)}
                />
              </Box>
              <Box width="500px" mb={6}>
                <FieldInput
                  label="Endpoint Port"
                  rule={requirePort}
                  value={dbPort}
                  placeholder="5432"
                  onChange={e => setDbPort(e.target.value)}
                />
              </Box>
              {/* TODO (lisa or ryan): add AWS input fields */}
              <Box>
                <Text bold>Labels (optional)</Text>
                <Text mb={2}>
                  Labels make this new database discoverable by the database
                  server. <br />
                  Not defining labels is equivalent to asteriks (any database
                  server can discover this database).
                </Text>
                <LabelsCreater
                  labels={labels}
                  setLabels={setLabels}
                  isLabelOptional={true}
                  disableBtns={attempt.status === 'processing'}
                />
              </Box>
            </>
          )}
          <ActionButtons
            onProceed={() => handleOnProceed(validator)}
            // On failure, allow user to attempt again.
            disableProceed={
              attempt.status === 'processing' || !canCreateDatabase
            }
          />
          {(attempt.status === 'processing' || attempt.status === 'failed') && (
            <CreateDatabaseDialog
              pollTimeout={pollTimeout}
              attempt={attempt}
              retry={() => handleOnProceed(validator)}
              close={clearAttempt}
            />
          )}
        </Box>
      )}
    </Validation>
  );
}

const CreateDatabaseDialog = ({
  pollTimeout,
  attempt,
  retry,
  close,
}: {
  pollTimeout: number;
  attempt: Attempt;
  retry(): void;
  close(): void;
}) => {
  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogContent
        width="400px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        {attempt.status !== 'failed' ? (
          <>
            {' '}
            <Text bold caps mb={4}>
              Registering Database
            </Text>
            <AnimatedProgressBar />
            <TextIcon
              css={`
                white-space: pre;
              `}
            >
              <Icons.Restore fontSize={4} />
              <Timeout
                timeout={pollTimeout}
                message=""
                tailMessage={' seconds left'}
              />
            </TextIcon>
          </>
        ) : (
          <Box width="100%">
            <Text bold caps mb={3}>
              Database Register Failed
            </Text>
            <Text mb={5}>
              <Icons.Warning ml={1} mr={2} color="danger" />
              Error: {attempt.statusText}
            </Text>
            <Flex>
              <ButtonPrimary mr={2} width="50%" onClick={retry}>
                Retry
              </ButtonPrimary>
              <ButtonSecondary width="50%" onClick={close}>
                Close
              </ButtonSecondary>
            </Flex>
          </Box>
        )}
      </DialogContent>
    </Dialog>
  );
};

// PORT_REGEXP only allows digits with length 4.
export const PORT_REGEX = /^\d{4}$/;
const requirePort = value => () => {
  const isValidId = value.match(PORT_REGEX);
  if (!isValidId) {
    return {
      valid: false,
      message: 'port must be 4 digits',
    };
  }
  return {
    valid: true,
  };
};
