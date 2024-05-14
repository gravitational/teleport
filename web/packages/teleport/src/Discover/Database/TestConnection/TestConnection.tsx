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

import React, { useState } from 'react';
import { Text, Box, LabelInput } from 'design';

import Select, { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { generateTshLoginCommand } from 'teleport/lib/util';
import ReAuthenticate from 'teleport/components/ReAuthenticate';
import { CustomInputFieldForAsterisks } from 'teleport/Discover/Shared/CustomInputFieldForAsterisks';

import { MfaChallengeScope } from 'teleport/services/auth/auth';
import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { MfaAuthnResponse } from 'teleport/services/mfa';
import { WILD_CARD } from 'teleport/Discover/Shared/const';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  ConnectionDiagnosticResult,
  StyledBox,
  useConnectionDiagnostic,
} from '../../Shared';
import { DatabaseEngine } from '../../SelectResource';

export function TestConnection() {
  const { resourceSpec, agentMeta } = useDiscover();
  const { engine: dbEngine } = resourceSpec.dbMeta;
  const db = (agentMeta as DbMeta).db;
  const { clusterId } = useStickyClusterId();

  const {
    runConnectionDiagnostic,
    attempt,
    diagnosis,
    nextStep,
    prevStep,
    canTestConnection,
    username,
    authType,
    showMfaDialog,
    cancelMfaDialog,
  } = useConnectionDiagnostic();

  const userOpts = db.users.map(l => ({ value: l, label: l }));
  const nameOpts = db.names.map(l => ({ value: l, label: l }));

  // These fields will never be empty as the previous step prevents users
  // from getting to this step if both are not defined.
  const [selectedDbUser, setSelectedDbUser] = useState(userOpts[0]);
  const [selectedDbName, setSelectedDbName] = useState(nameOpts[0]);

  // customs is only allowed if user selected an user/name option
  // that is an "asteriks".
  const [customDbUser, setCustomDbUser] = useState('');
  const [customDbName, setCustomDbName] = useState('');

  const dbUser = getInputValue(customDbUser || selectedDbUser.value, 'user');
  let tshDbCmd = `tsh db connect ${db.name} --db-user=${dbUser}`;

  if (customDbName || selectedDbName) {
    const dbName = getInputValue(customDbName || selectedDbName.value, 'name');
    tshDbCmd += ` --db-name=${dbName}`;
  }

  function makeTestConnRequest() {
    return {
      name: customDbName || selectedDbName?.value,
      user: customDbUser || selectedDbUser?.value,
    };
  }

  function testConnection(
    validator: Validator,
    mfaResponse?: MfaAuthnResponse
  ) {
    if (!validator.validate()) {
      return;
    }
    if (!customDbName && !customDbUser) {
      validator.reset();
    }
    runConnectionDiagnostic(
      {
        resourceKind: 'db',
        resourceName: agentMeta.resourceName,
        dbTester: makeTestConnRequest(),
      },
      mfaResponse
    );
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box>
          {showMfaDialog && (
            <ReAuthenticate
              onMfaResponse={res => testConnection(validator, res)}
              onClose={cancelMfaDialog}
              challengeScope={MfaChallengeScope.USER_SESSION}
            />
          )}
          <Header>Test Connection</Header>
          <HeaderSubtitle>
            Optionally verify that you can successfully connect to the Database
            you just added.
          </HeaderSubtitle>
          <StyledBox mb={5}>
            <Text bold>Step 1</Text>
            <Text typography="subtitle1" mb={3}>
              Select a user and a database name to test.
            </Text>
            <Box width="500px" mb={4}>
              <LabelInput htmlFor={'select'}>Database User</LabelInput>
              <Select
                data-testid="select-db-user"
                placeholder={
                  userOpts.length === 0
                    ? 'No database users defined'
                    : 'Click to select a database user'
                }
                isSearchable
                value={selectedDbUser}
                onChange={(o: Option) => {
                  setSelectedDbUser(o);
                  if (customDbUser && o?.value !== WILD_CARD) {
                    setCustomDbUser('');
                  }
                }}
                options={userOpts}
                isDisabled={
                  attempt.status === 'processing' || userOpts.length === 0
                }
              />
              <CustomInputFieldForAsterisks
                selectedOption={selectedDbUser}
                value={customDbUser}
                onValueChange={setCustomDbUser}
                disabled={attempt.status === 'processing'}
                nameKind="database user"
              />
            </Box>
            <Box width="500px" mb={3}>
              <LabelInput htmlFor={'select'}>Database Name</LabelInput>
              <Select
                data-testid="select-db-name"
                label="Database Name"
                placeholder={
                  nameOpts.length === 0
                    ? 'No database names defined'
                    : 'Click to select a database name'
                }
                isSearchable
                value={selectedDbName}
                onChange={(o: Option) => {
                  setSelectedDbName(o);
                  if (customDbName && o?.value !== WILD_CARD) {
                    setCustomDbName('');
                  }
                }}
                options={nameOpts}
                isDisabled={
                  attempt.status === 'processing' || nameOpts.length === 0
                }
                // Database name is required for Postgres.
                isClearable={dbEngine !== DatabaseEngine.Postgres}
              />
              <CustomInputFieldForAsterisks
                selectedOption={selectedDbName}
                value={customDbName}
                onValueChange={setCustomDbName}
                disabled={attempt.status === 'processing'}
                nameKind="database"
              />
            </Box>
          </StyledBox>
          <ConnectionDiagnosticResult
            attempt={attempt}
            diagnosis={diagnosis}
            canTestConnection={canTestConnection}
            testConnection={() => testConnection(validator)}
            stepNumber={2}
            stepDescription="Verify that your database is accessible"
          />
          <StyledBox>
            <Text bold mb={3}>
              To Access your Database
            </Text>
            <Box mb={2}>
              Log into your Teleport cluster
              <TextSelectCopy
                mt="1"
                text={generateTshLoginCommand({
                  authType,
                  username,
                  clusterId,
                })}
              />
            </Box>
            <Box mb={2}>
              Connect to your database
              <TextSelectCopy mt="1" text={tshDbCmd} />
            </Box>
          </StyledBox>
          <ActionButtons
            onProceed={nextStep}
            lastStep={true}
            onPrev={prevStep}
          />
        </Box>
      )}
    </Validation>
  );
}

function getInputValue(input: string, inputKind: 'name' | 'user') {
  if (input == WILD_CARD) {
    return inputKind === 'name' ? '<name>' : '<user>';
  }
  return input;
}
