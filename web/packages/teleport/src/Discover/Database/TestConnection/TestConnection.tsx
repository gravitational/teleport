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

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { generateTshLoginCommand } from 'teleport/lib/util';
import ReAuthenticate from 'teleport/components/ReAuthenticate';

import { MfaChallengeScope } from 'teleport/services/auth/auth';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  ConnectionDiagnosticResult,
  StyledBox,
} from '../../Shared';
import { DatabaseEngine } from '../../SelectResource';

import { useTestConnection, State } from './useTestConnection';

import type { AgentStepProps } from '../../types';

/**
 * @deprecated Refactor Discover/Database/TestConnection away from the container component
 * pattern. See https://github.com/gravitational/teleport/pull/34952.
 */
export function TestConnection(props: AgentStepProps) {
  const state = useTestConnection(props);

  return <TestConnectionView {...state} />;
}

export function TestConnectionView({
  attempt,
  testConnection,
  diagnosis,
  nextStep,
  prevStep,
  canTestConnection,
  db,
  authType,
  username,
  clusterId,
  dbEngine,
  showMfaDialog,
  cancelMfaDialog,
}: State) {
  const userOpts = db.users.map(l => ({ value: l, label: l }));
  const nameOpts = db.names.map(l => ({ value: l, label: l }));

  // These fields will never be empty as the previous step prevents users
  // from getting to this step if both are not defined.
  const [selectedUser, setSelectedUser] = useState(userOpts[0]);
  const [selectedName, setSelectedName] = useState(nameOpts[0]);

  let tshDbCmd = `tsh db connect ${db.name} --db-user=${selectedUser.value}`;
  if (selectedName) {
    tshDbCmd += ` --db-name=${selectedName.value}`;
  }

  function makeTestConnRequest() {
    return {
      name: selectedName?.value,
      user: selectedUser?.value,
    };
  }

  return (
    <Box>
      {showMfaDialog && (
        <ReAuthenticate
          onMfaResponse={res => testConnection(makeTestConnRequest(), res)}
          onClose={cancelMfaDialog}
          challengeScope={MfaChallengeScope.USER_SESSION}
        />
      )}
      <Header>Test Connection</Header>
      <HeaderSubtitle>
        Optionally verify that you can successfully connect to the Database you
        just added.
      </HeaderSubtitle>
      <StyledBox mb={5}>
        <Text bold>Step 1</Text>
        <Text typography="subtitle1" mb={3}>
          Select a user and a database name to test.
        </Text>
        <Box width="500px" mb={4}>
          <LabelInput htmlFor={'select'}>Database User</LabelInput>
          <Select
            placeholder={
              userOpts.length === 0
                ? 'No database users defined'
                : 'Click to select a database user'
            }
            isSearchable
            value={selectedUser}
            onChange={(o: Option) => setSelectedUser(o)}
            options={userOpts}
            isDisabled={
              attempt.status === 'processing' || userOpts.length === 0
            }
          />
        </Box>
        <Box width="500px" mb={3}>
          <LabelInput htmlFor={'select'}>Database Name</LabelInput>
          <Select
            label="Database Name"
            placeholder={
              nameOpts.length === 0
                ? 'No database names defined'
                : 'Click to select a database name'
            }
            isSearchable
            value={selectedName}
            onChange={(o: Option) => setSelectedName(o)}
            options={nameOpts}
            isDisabled={
              attempt.status === 'processing' || nameOpts.length === 0
            }
            // Database name is required for Postgres.
            isClearable={dbEngine !== DatabaseEngine.Postgres}
          />
        </Box>
      </StyledBox>
      <ConnectionDiagnosticResult
        attempt={attempt}
        diagnosis={diagnosis}
        canTestConnection={canTestConnection}
        testConnection={() => testConnection(makeTestConnRequest())}
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
      <ActionButtons onProceed={nextStep} lastStep={true} onPrev={prevStep} />
    </Box>
  );
}
