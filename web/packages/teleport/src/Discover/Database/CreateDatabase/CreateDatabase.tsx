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

import { useEffect, useState } from 'react';

import { Box, Flex, Mark, Text } from 'design';
import { P } from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput';
import TextEditor from 'shared/components/TextEditor';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { ResourceLabelTooltip } from 'teleport/Discover/Shared/ResourceLabelTooltip';
import type { ResourceLabel } from 'teleport/services/agents';

import {
  getDatabaseProtocol,
  getDefaultDatabasePort,
} from '../../SelectResource';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  LabelsCreater,
} from '../../Shared';
import { dbCU } from '../../yamlTemplates';
import { CreateDatabaseDialog } from './CreateDatabaseDialog';
import { State, useCreateDatabase } from './useCreateDatabase';

export function CreateDatabase() {
  const state = useCreateDatabase();
  return <CreateDatabaseView {...state} />;
}

export function CreateDatabaseView({
  attempt,
  clearAttempt,
  registerDatabase,
  canCreateDatabase,
  pollTimeout,
  dbEngine,
  isDbCreateErr,
  prevStep,
  nextStep,
  handleOnTimeout,
}: State) {
  const [dbName, setDbName] = useState('');
  const [dbUri, setDbUri] = useState('');
  const [labels, setLabels] = useState<ResourceLabel[]>([]);
  const [dbPort, setDbPort] = useState(getDefaultDatabasePort(dbEngine));

  const [finishedFirstStep, setFinishedFirstStep] = useState(false);

  useEffect(() => {
    // If error resulted from creating a db, reset the view
    // to the beginning as the error could be from duplicate
    // db name.
    if (isDbCreateErr) {
      setFinishedFirstStep(false);
    }
  }, [isDbCreateErr]);

  function handleOnProceed(
    validator: Validator,
    { overwriteDb = false, retry = false } = {}
  ) {
    if (!validator.validate()) {
      return;
    }

    if (!retry && !finishedFirstStep) {
      setFinishedFirstStep(true);
      validator.reset();
      return;
    }

    registerDatabase(
      {
        labels,
        name: dbName,
        uri: `${dbUri}:${dbPort}`,
        protocol: getDatabaseProtocol(dbEngine),
      },
      { overwriteDb }
    );
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
              <P>You don't have permission to register a database.</P>
              <P>
                Please ask your Teleport administrator to update your role and
                add the <Mark>db</Mark> rule:
              </P>
              <Flex minHeight="195px" mt={3}>
                <TextEditor
                  readOnly={true}
                  bg="levels.deep"
                  data={[{ content: dbCU, type: 'yaml' }]}
                />
              </Flex>
            </Box>
          )}
          {canCreateDatabase && (
            <>
              {!finishedFirstStep && (
                <Box width="500px">
                  <FieldInput
                    label="Database Name"
                    rule={requiredField('database name is required')}
                    autoFocus
                    value={dbName}
                    placeholder="Enter database name"
                    onChange={e => setDbName(e.target.value)}
                    toolTipContent="An identifier name for this new database for Teleport."
                  />
                </Box>
              )}
              {finishedFirstStep && (
                <>
                  <Flex width="500px">
                    <FieldInput
                      autoFocus
                      label="Database Connection Endpoint"
                      rule={requiredField('connection endpoint is required')}
                      value={dbUri}
                      placeholder="db.example.com"
                      onChange={e => setDbUri(e.target.value)}
                      width="70%"
                      mr={2}
                      toolTipContent="Database location and connection information."
                    />
                    <FieldInput
                      label="Endpoint Port"
                      rule={requirePort}
                      value={dbPort}
                      placeholder="5432"
                      onChange={e => setDbPort(e.target.value)}
                      width="30%"
                    />
                  </Flex>
                  <Box mt={3}>
                    <Flex alignItems="center" gap={1} mb={2}>
                      <Text bold>Labels (optional)</Text>
                      <ResourceLabelTooltip
                        toolTipPosition="top"
                        resourceKind="db"
                      />
                    </Flex>
                    <LabelsCreater
                      labels={labels}
                      setLabels={setLabels}
                      isLabelOptional={true}
                      disableBtns={attempt.status === 'processing'}
                      noDuplicateKey={true}
                    />
                  </Box>
                </>
              )}
            </>
          )}
          <ActionButtons
            onPrev={prevStep}
            onProceed={() => handleOnProceed(validator)}
            // On failure, allow user to attempt again.
            disableProceed={
              attempt.status === 'processing' || !canCreateDatabase
            }
          />
          {attempt.status !== '' && (
            <CreateDatabaseDialog
              pollTimeout={pollTimeout}
              attempt={attempt}
              retry={() => handleOnProceed(validator, { retry: true })}
              onOverwrite={() =>
                handleOnProceed(validator, { overwriteDb: true })
              }
              onTimeout={handleOnTimeout}
              close={clearAttempt}
              dbName={dbName}
              next={nextStep}
            />
          )}
        </Box>
      )}
    </Validation>
  );
}

// Only allows digits with valid port range 1-65535.
const requirePort = (value: string) => () => {
  const numberValue = parseInt(value);
  const isValidPort =
    Number.isInteger(numberValue) && numberValue >= 1 && numberValue <= 65535;
  if (!isValidPort) {
    return {
      valid: false,
      message: 'invalid port (1-65535)',
    };
  }
  return {
    valid: true,
  };
};
