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
import styled from 'styled-components';
import { Text, Box } from 'design';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { generateTshLoginCommand } from 'teleport/lib/util';
import ReAuthenticate from 'teleport/components/ReAuthenticate';

import {
  ActionButtons,
  HeaderSubtitle,
  Header,
  ConnectionDiagnosticResult,
} from '../../Shared';

import { useTestConnection, State } from './useTestConnection';

import type { AgentStepProps } from '../../types';
import type { KubeImpersonation } from 'teleport/services/agents';

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
              onMfaResponse={res => testConnection(makeTestConnRequest(), res)}
              onClose={cancelMfaDialog}
            />
          )}
          <Header>Test Connection</Header>
          <HeaderSubtitle>
            Optionally verify that you can successfully connect to the
            Kubernetes cluster you just added.
          </HeaderSubtitle>
          <StyledBox mb={5}>
            <Text bold>Step 1</Text>
            <Text typography="subtitle1" mb={3}>
              Define the namespace to test.
            </Text>
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
            <Text bold>Step 2</Text>
            <Text typography="subtitle1" mb={3}>
              Select groups and a user to test.
            </Text>
            <Box width="500px">
              <FieldSelect
                label="Kubernetes groups"
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
                label={'Kubernetes user'}
                labelTip={
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
            <Text bold mb={3}>
              To Access your Kubernetes cluster
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

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  border-radius: 8px;
  padding: 20px;
`;
