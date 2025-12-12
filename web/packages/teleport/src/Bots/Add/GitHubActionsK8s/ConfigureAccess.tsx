/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import styled from 'styled-components';

import Box from 'design/Box/Box';
import { ButtonPrimary, ButtonSecondary } from 'design/Button/Button';
import Flex from 'design/Flex/Flex';
import Link from 'design/Link/Link';
import Text, { H2 } from 'design/Text';
import { FieldSelectCreatable } from 'shared/components/FieldSelect/FieldSelectCreatable';
import Validator, { Validation } from 'shared/components/Validation/Validation';

import { SectionBox } from 'teleport/Roles/RoleEditor/StandardEditor/sections';
import {
  IntegrationEnrollField,
  IntegrationEnrollSection,
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';

import { FlowStepProps } from '../Shared/GuidedFlow';
import { useTracking } from '../Shared/useTracking';
import { CodePanel } from './CodePanel';
import { KubernetesLabelsSelect } from './KubernetesLabelsSelect';
import { useGitHubK8sFlow } from './useGitHubK8sFlow';

export function ConfigureAccess(props: FlowStepProps) {
  const { nextStep, prevStep } = props;

  const { dispatch, state } = useGitHubK8sFlow();
  const tracking = useTracking();

  const handleNext = (validator: Validator) => {
    if (!validator.validate()) {
      tracking.error(
        IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
        'validation error'
      );
      return;
    }

    tracking.step(
      IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
      IntegrationEnrollStatusCode.Success
    );

    nextStep?.();
  };

  return (
    <Container>
      <FormContainer>
        <Box>
          <H2 mb={3} mt={3}>
            Configure Access
          </H2>

          <Text mb={3}>
            Fine tune the access your workflow needs to perform its steps.
            Restrict which clusters can be accessed using labels and what level
            of access using the other options.
          </Text>
        </Box>

        <Validation>
          {({ validator }) => (
            <>
              <div>
                <KubernetesLabelsSelect
                  mt={2}
                  selected={state.kubernetesLabels}
                  onChange={labels => {
                    dispatch({
                      type: 'kubernetes-labels-changed',
                      value: labels,
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
                      IntegrationEnrollField.MWIGHAK8SKubernetesLabels
                    );
                  }}
                />
                <Text mt={3} mb={3}>
                  Your workflow will have access to Kubernetes clusters which
                  satisfy the labels you specify. Visit the{' '}
                  <Link
                    target="_blank"
                    href={K8S_RBAC_LINK}
                    onClick={() => {
                      tracking.link(
                        IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
                        K8S_RBAC_LINK
                      );
                    }}
                  >
                    Teleport Kubernetes Access Controls
                  </Link>{' '}
                  docs for information about using Kubernetes labels.
                </Text>

                <FieldSelectCreatable
                  label="Kubernetes Groups"
                  mt={2}
                  placeholder="e.g. system:masters"
                  isMulti
                  value={state.kubernetesGroups.map(g => ({
                    label: g,
                    value: g,
                  }))}
                  onChange={e => {
                    dispatch({
                      type: 'kubernetes-groups-changed',
                      value: e.map(g => g.value),
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
                      IntegrationEnrollField.MWIGHAK8SKubernetesGroups
                    );
                  }}
                  createOptionPosition="last"
                />
                <Text mb={3}>
                  Add Kubernetes groups created using RoleBinding or
                  ClusterRoleBinding resources. See the{' '}
                  <Link
                    target="_blank"
                    href={K8S_RBAC_LINK}
                    onClick={() => {
                      tracking.link(
                        IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
                        K8S_RBAC_LINK
                      );
                    }}
                  >
                    Teleport Kubernetes Access Controls
                  </Link>{' '}
                  docs for information about mapping groups to roles.
                </Text>
              </div>

              {/* TODO(nicholasmarais1158): Make SectionBox a component instead of reusing it from Roles */}
              <SectionBox
                titleSegments={['Advanced options']}
                initiallyCollapsed={
                  [
                    'workflow',
                    'environment',
                    'enterpriseSlug',
                    'enterpriseJwks',
                  ].every(k => !state[k]) && state.refType === 'branch'
                }
                validation={{
                  valid: true,
                }}
                onExpand={() => {
                  tracking.section(
                    IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
                    IntegrationEnrollSection.MWIGHAK8SKubernetesAdvancedOptions
                  );
                }}
              >
                <FieldSelectCreatable
                  label="Kubernetes Users"
                  mt={2}
                  isMulti
                  placeholder={'e.g. user@example.com'}
                  value={state.kubernetesUsers.map(g => ({
                    label: g,
                    value: g,
                  }))}
                  onChange={e => {
                    dispatch({
                      type: 'kubernetes-users-changed',
                      value: e.map(g => g.value),
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConfigureAccess,
                      IntegrationEnrollField.MWIGHAK8SKubernetesUsers
                    );
                  }}
                  createOptionPosition="last"
                />
                <Text mb={3}>
                  See the{' '}
                  <Link
                    target="_blank"
                    href="https://goteleport.com/docs/enroll-resources/kubernetes-access/controls/"
                  >
                    Teleport Kubernetes Access Controls
                  </Link>{' '}
                  docs for more information about using users and service
                  accounts.
                </Text>
              </SectionBox>

              <Flex gap={2} pt={5}>
                <ButtonPrimary onClick={() => handleNext(validator)}>
                  Next
                </ButtonPrimary>
                <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
              </Flex>
            </>
          )}
        </Validation>
      </FormContainer>

      <CodeContainer>
        <CodePanel />
      </CodeContainer>
    </Container>
  );
}

const Container = styled(Flex)`
  flex: 1;
  overflow: auto;
  gap: ${({ theme }) => theme.space[1]}px;
`;

const FormContainer = styled(Flex)`
  flex: 1;
  flex-direction: column;
  overflow: auto;
  padding-right: ${({ theme }) => theme.space[5]}px;
`;

const CodeContainer = styled(Flex)`
  flex: 1;
  flex-direction: column;
  overflow: auto;
`;

const K8S_RBAC_LINK =
  'https://goteleport.com/docs/enroll-resources/kubernetes-access/controls/';
