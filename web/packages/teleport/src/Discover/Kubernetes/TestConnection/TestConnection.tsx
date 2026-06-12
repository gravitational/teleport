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

import { useState } from 'react';

import { Box, H3, Subtitle3 } from 'design';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import ReAuthenticate from 'teleport/components/ReAuthenticate';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { generateTshLoginCommand } from 'teleport/lib/util';
import type { KubeImpersonation } from 'teleport/services/agents';
import { MfaChallengeScope } from 'teleport/services/auth/auth';

import {
  ActionButtons,
  ConnectionDiagnosticResult,
  Header,
  HeaderSubtitle,
  StyledBox,
} from '../../Shared';
import type { AgentStepProps } from '../../types';
import { State, useTestConnection } from './useTestConnection';

/**
 * @deprecated Refactor Discover/Kubernetes/TestConnection away from the container component
 * pattern. See https://github.com/gravitational/teleport/pull/34952.
 */
export default function Container(props: AgentStepProps) {
  const state = useTestConnection(props);

  return <TestConnection {...state} />;
}

export function TestConnection({
  attempt,
  testConnection,
  diagnosis,
  nextStep,
  prevStep,
  canTestConnection,
  kube,
  authType,
  username,
  clusterId,
  showMfaDialog,
  cancelMfaDialog,
}: State) {
  const userOpts = kube.users.map(l => ({ value: l, label: l }));
  const groupOpts = kube.groups.map(l => ({ value: l, label: l }));

  const [namespace, setNamespace] = useState('default');
  const [selectedGroups, setSelectedGroups] = useState(groupOpts);

  // Always default it to either teleport username or from one of users defined
  // from previous step.
  const [selectedUser, setSelectedUser] = useState(
    () => userOpts[0] || { value: username, label: username }
  );

  function handleTestConnection(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    testConnection(makeTestConnRequest());
  }

  function makeTestConnRequest(): KubeImpersonation {
    return {
      namespace,
      user: selectedUser?.value,
      groups: selectedGroups?.map(g => g.value),
    };
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box>
          {showMfaDialog && (
            <ReAuthenticate
              onMfaResponse={async res =>
                testConnection(makeTestConnRequest(), res)
              }
              onClose={cancelMfaDialog}
              challengeScope={MfaChallengeScope.USER_SESSION}
            />
          )}
          <Header>Test Connection</Header>
          <HeaderSubtitle>
            Optionally verify that you can successfully connect to the
            Kubernetes cluster you just added.
          </HeaderSubtitle>
          <StyledBox mb={5}>
            <header>
              <H3>Step 1</H3>
              <Subtitle3 mb={3}>Define the namespace to test.</Subtitle3>
            </header>
            <Box width="500px">
              <FieldInput
                label="Namespace"
                rule={requiredField('Namespace is required')}
                autoFocus
                value={namespace}
                placeholder="Enter namespace"
                onChange={e => setNamespace(e.target.value)}
              />
            </Box>
          </StyledBox>
          <StyledBox mb={5}>
            <header>
              <H3>Step 2</H3>
              <Subtitle3 mb={3}>Select groups and a user to test.</Subtitle3>
            </header>
            <Box width="500px">
              <FieldSelect
                label="Kubernetes Groups"
                placeholder={
                  groupOpts.length === 0
                    ? 'No groups defined'
                    : 'Click to select groups'
                }
                isSearchable
                isMulti
                isClearable={false}
                value={selectedGroups}
                onChange={values => setSelectedGroups(values as Option[])}
                options={groupOpts}
                isDisabled={
                  attempt.status === 'processing' || groupOpts.length === 0
                }
              />
            </Box>
            <Box width="500px">
              <FieldSelect
                label={'Kubernetes User'}
                helperText={
                  userOpts.length === 0
                    ? 'Defaulted to your teleport username'
                    : ''
                }
                isSearchable
                isClearable={true}
                placeholder="Select a user"
                value={selectedUser}
                onChange={(o: Option) => setSelectedUser(o)}
                options={userOpts}
                isDisabled={
                  attempt.status === 'processing' || userOpts.length === 0
                }
              />
            </Box>
          </StyledBox>
          <ConnectionDiagnosticResult
            attempt={attempt}
            diagnosis={diagnosis}
            canTestConnection={canTestConnection}
            testConnection={() => handleTestConnection(validator)}
            stepNumber={3}
            stepDescription="Verify that the Kubernetes is accessible"
          />
          <StyledBox>
            <H3 mb={3}>To Access your Kubernetes cluster</H3>
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
              Log into your Kubernetes cluster
              <TextSelectCopy mt="1" text={`tsh kube login ${kube.name}`} />
            </Box>
            <Box>
              Use kubectl
              <TextSelectCopy mt="1" text="kubectl get pods" />
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
