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

import React, { useState, useEffect } from 'react';
import { Text, Box, Flex, Mark } from 'design';

import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import TextEditor from 'shared/components/TextEditor';

import {
  ActionButtons,
  HeaderSubtitle,
  LabelsCreater,
  Header,
} from '../../Shared';
import { dbCU } from '../../yamlTemplates';
import {
  getDatabaseProtocol,
  getDefaultDatabasePort,
} from '../../SelectResource';

import { useCreateDatabase, State } from './useCreateDatabase';
import { CreateDatabaseDialog } from './CreateDatabaseDialog';

import type { ResourceLabel } from 'teleport/services/agents';

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

  function handleOnProceed(validator: Validator, retry = false) {
    if (!validator.validate()) {
      return;
    }

    if (!retry && !finishedFirstStep) {
      setFinishedFirstStep(true);
      validator.reset();
      return;
    }

    registerDatabase({
      labels,
      name: dbName,
      uri: `${dbUri}:${dbPort}`,
      protocol: getDatabaseProtocol(dbEngine),
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
                    <Text bold>Labels (optional)</Text>
                    <Text mb={2}>
                      Labels make this new database discoverable by the database
                      service. <br />
                      Not defining labels is equivalent to asterisks (any
                      database service can discover this database).
                    </Text>
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
              retry={() => handleOnProceed(validator, true /* retry */)}
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
