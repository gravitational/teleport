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

import { Box, H2, Link, Text } from 'design';
import { Info } from 'design/Alert/Alert';
import { ButtonPrimary, ButtonSecondary } from 'design/Button/Button';
import Flex from 'design/Flex/Flex';
import { FieldCheckbox } from 'shared/components/FieldCheckbox/FieldCheckbox';
import FieldInput from 'shared/components/FieldInput/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect/FieldSelect';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { SectionBox } from 'teleport/Roles/RoleEditor/StandardEditor/sections';
import {
  IntegrationEnrollField,
  IntegrationEnrollSection,
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';

import {
  RefTypeOption,
  refTypeOptions,
  requireValidRepository,
} from '../Shared/github';
import { FlowStepProps } from '../Shared/GuidedFlow';
import { useTracking } from '../Shared/useTracking';
import { CodePanel } from './CodePanel';
import { useGitHubK8sFlow } from './useGitHubK8sFlow';

export function ConnectGitHub(props: FlowStepProps) {
  const { nextStep, prevStep } = props;

  const { dispatch, state } = useGitHubK8sFlow();

  const tracking = useTracking();

  const handleNext = (validator: Validator) => {
    if (!validator.validate()) {
      tracking.error(
        IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
        'validation error'
      );

      return;
    }

    tracking.step(
      IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
      IntegrationEnrollStatusCode.Success
    );

    nextStep?.();
  };

  const refTypeValue: RefTypeOption =
    refTypeOptions.find(o => o.value === state.refType) ?? refTypeOptions[0];

  return (
    <Container>
      <FormContainer>
        <Box>
          <H2 mb={3} mt={3}>
            Connect to GitHub
          </H2>

          <Text mb={3}>
            Provide details for the GitHub repository you would like to connect.
          </Text>
        </Box>

        <Validation>
          {({ validator }) => (
            <div>
              <FieldInput
                rule={requireValidRepository}
                label="Repository URL"
                placeholder="https://github.com/gravitational/teleport"
                value={state.gitHubUrl}
                onChange={e => {
                  dispatch({
                    type: 'github-url-changed',
                    value: e.target.value,
                  });
                  tracking.field(
                    IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                    IntegrationEnrollField.MWIGHAK8SGitHubRepositoryURL,
                    !e.target.value.length
                  );
                }}
              />

              <FieldInput
                rule={
                  state.isBranchDisabled || state.allowAnyBranch
                    ? undefined
                    : requiredField('A branch is required')
                }
                disabled={state.isBranchDisabled}
                label="Branch"
                placeholder="main"
                value={state.branch}
                onChange={e => {
                  dispatch({
                    type: 'branch-changed',
                    value: e.target.value,
                  });
                  tracking.field(
                    IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                    IntegrationEnrollField.MWIGHAK8SGitHubBranch,
                    !e.target.value.length
                  );
                }}
              />
              <FieldCheckbox
                label="Allow any branch"
                checked={state.allowAnyBranch}
                disabled={state.isBranchDisabled}
                onChange={e =>
                  dispatch({
                    type: 'allow-any-branch-toggled',
                    value: e.target.checked,
                  })
                }
                size="small"
              />
              <Text mb={3}>
                Specifying a branch is recommended to prevent workflows running
                from any branch to access your Teleport resources.
              </Text>

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
                    IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                    IntegrationEnrollSection.MWIGHAK8SGitHubAdvancedOptions
                  );
                }}
              >
                <FieldInput
                  label="Workflow"
                  placeholder="my-workflow"
                  value={state.workflow}
                  onChange={e => {
                    dispatch({
                      type: 'workflow-changed',
                      value: e.target.value,
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                      IntegrationEnrollField.MWIGHAK8SGitHubWorkflow,
                      !e.target.value.length
                    );
                  }}
                />

                <FieldInput
                  label="Environment"
                  placeholder="production"
                  value={state.environment}
                  onChange={e => {
                    dispatch({
                      type: 'environment-changed',
                      value: e.target.value,
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                      IntegrationEnrollField.MWIGHAK8SGitHubEnvironment,
                      !e.target.value.length
                    );
                  }}
                />

                <Flex mb={3} gap={2}>
                  <FieldInput
                    flex={1}
                    label={'Git Ref'}
                    placeholder="ref/heads/main"
                    value={state.ref}
                    onChange={e => {
                      dispatch({
                        type: 'ref-changed',
                        value: e.target.value,
                      });
                      tracking.field(
                        IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                        IntegrationEnrollField.MWIGHAK8SGitHubRef,
                        !e.target.value.length
                      );
                    }}
                    borderTopRightRadius={0}
                    borderBottomRightRadius={0}
                  />
                  <FieldSelect
                    width={160}
                    label="Ref Type"
                    isMulti={false}
                    value={refTypeValue}
                    onChange={o => {
                      dispatch({
                        type: 'ref-type-changed',
                        value: o?.value ?? '',
                      });
                      tracking.field(
                        IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                        IntegrationEnrollField.MWIGHAK8SGitHubRef
                      );
                    }}
                    options={refTypeOptions}
                    menuPlacement="auto"
                  />
                </Flex>

                {cfg.edition !== 'ent' ? (
                  <Info
                    mt={5}
                    mb={3}
                    alignItems="flex-start"
                    details={
                      <>
                        GitHub Enterprise Server configuration requires Teleport
                        Enterprise. Please use a repository hosted at github.com
                        or{' '}
                        <Link
                          target="_blank"
                          href="https://goteleport.com/signup/enterprise/"
                        >
                          contact us
                        </Link>
                        .
                      </>
                    }
                  >
                    GitHub Enterprise Server
                  </Info>
                ) : undefined}

                <FieldInput
                  label="Enterprise slug"
                  disabled={cfg.edition !== 'ent'}
                  placeholder="octo-enterprise"
                  value={state.enterpriseSlug}
                  onChange={e => {
                    dispatch({
                      type: 'slug-changed',
                      value: e.target.value,
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                      IntegrationEnrollField.MWIGHAK8SGitHubEnterpriseSlug,
                      !e.target.value.length
                    );
                  }}
                />

                <FieldInput
                  label="Enterprise JWKS"
                  disabled={cfg.edition !== 'ent'}
                  placeholder='{"keys":[ --snip-- ]}'
                  value={state.enterpriseJwks}
                  onChange={e => {
                    dispatch({
                      type: 'jwks-changed',
                      value: e.target.value,
                    });
                    tracking.field(
                      IntegrationEnrollStep.MWIGHAK8SConnectGitHub,
                      IntegrationEnrollField.MWIGHAK8SGitHubEnterpriseStaticJWKS,
                      !e.target.value.length
                    );
                  }}
                />
              </SectionBox>

              <Flex gap={2} pt={5}>
                <ButtonPrimary onClick={() => handleNext(validator)}>
                  Next
                </ButtonPrimary>
                <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
              </Flex>
            </div>
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
