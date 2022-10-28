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
import { ButtonSecondary, Text, Box, Flex, ButtonText } from 'design';
import * as Icons from 'design/Icon';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import useTeleport from 'teleport/useTeleport';
import { generateTshLoginCommand } from 'teleport/lib/util';

import {
  Header,
  ActionButtons,
  TextIcon,
  HeaderSubtitle,
  Mark,
  ReadOnlyYamlEditor,
} from '../../Shared';
import { ruleConnectionDiagnostic } from '../../templates';

import { useTestConnection, State } from './useTestConnection';

import type { AgentStepProps } from '../../types';

export default function Container(props: AgentStepProps) {
  const ctx = useTeleport();
  const state = useTestConnection({ ctx, props });

  return <TestConnection {...state} />;
}

export function TestConnection({
  attempt,
  runConnectionDiagnostic,
  diagnosis,
  nextStep,
  canTestConnection,
  kube,
  authType,
  username,
  clusterId,
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

  let $diagnosisStateComponent;
  if (attempt.status === 'processing') {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Restore fontSize={4} />
        Testing in-progress
      </TextIcon>
    );
  } else if (attempt.status === 'failed' || (diagnosis && !diagnosis.success)) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.Warning ml={1} color="danger" />
        Testing failed
      </TextIcon>
    );
  } else if (attempt.status === 'success' && diagnosis?.success) {
    $diagnosisStateComponent = (
      <TextIcon>
        <Icons.CircleCheck ml={1} color="success" />
        Testing complete
      </TextIcon>
    );
  }

  const showDiagnosisOutput = !!diagnosis || attempt.status === 'failed';

  function handleTestConnection(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    runConnectionDiagnostic({
      namespace,
      user: selectedUser?.value,
      groups: selectedGroups?.map(g => g.value),
    });
  }

  return (
    <Validation>
      {({ validator }) => (
        <Box>
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
          <StyledBox mb={5}>
            <Text bold>Step 3</Text>
            <Text typography="subtitle1" mb={3}>
              Verify that the Kubernetes is accessible
            </Text>
            <Flex alignItems="center" mt={3}>
              {canTestConnection ? (
                <>
                  <ButtonSecondary
                    width="200px"
                    onClick={() => handleTestConnection(validator)}
                    disabled={attempt.status === 'processing'}
                  >
                    {diagnosis ? 'Restart Test' : 'Test Connection'}
                  </ButtonSecondary>
                  <Box ml={4}>{$diagnosisStateComponent}</Box>
                </>
              ) : (
                <Box>
                  <Text>
                    You don't have permission to test connection.
                    <br />
                    Please ask your Teleport administrator to update your role
                    and add the <Mark>connection_diagnostic</Mark> rule:
                  </Text>
                  <Flex minHeight="190px" mt={3}>
                    <ReadOnlyYamlEditor content={ruleConnectionDiagnostic} />
                  </Flex>
                </Box>
              )}
            </Flex>
            {showDiagnosisOutput && (
              <Box mt={3}>
                {attempt.status === 'failed' &&
                  `Encountered Error: ${attempt.statusText}`}
                {attempt.status === 'success' && (
                  <Box>
                    {diagnosis.traces.map((trace, index) => {
                      if (trace.status === 'failed') {
                        return (
                          <ErrorWithDetails
                            error={trace.error}
                            details={trace.details}
                            key={index}
                          />
                        );
                      }
                      if (trace.status === 'success') {
                        return (
                          <TextIcon
                            key={index}
                            css={{ alignItems: 'baseline' }}
                          >
                            <Icons.CircleCheck mr={1} color="success" />
                            {trace.details}
                          </TextIcon>
                        );
                      }

                      // For whatever reason the status is not the value
                      // of failed or success.
                      return (
                        <TextIcon key={index}>
                          <Icons.Question mr={1} />
                          {trace.details}
                        </TextIcon>
                      );
                    })}
                  </Box>
                )}
              </Box>
            )}
          </StyledBox>
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
          <ActionButtons onProceed={nextStep} lastStep={true} />
        </Box>
      )}
    </Validation>
  );
}

const ErrorWithDetails = ({
  details,
  error,
}: {
  details: string;
  error: string;
}) => {
  const [showMore, setShowMore] = useState(false);
  return (
    <TextIcon css={{ alignItems: 'baseline' }}>
      <Icons.CircleCross mr={1} color="danger" />
      <div>
        <div>{details}</div>
        <div>
          <ButtonShowMore onClick={() => setShowMore(p => !p)}>
            {showMore ? 'Hide' : 'Click for extra'} details
          </ButtonShowMore>
          {showMore && <div>{error}</div>}
        </div>
      </div>
    </TextIcon>
  );
};

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 20px;
`;

const ButtonShowMore = styled(ButtonText)`
  min-height: auto;
  padding: 0;
  font-weight: inherit;
  text-decoration: underline;
`;
