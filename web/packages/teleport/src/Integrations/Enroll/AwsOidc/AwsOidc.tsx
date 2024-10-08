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

import { Box, ButtonPrimary, ButtonSecondary, Flex, Link, Text } from 'design';
import * as Icons from 'design/Icon';
import { Link as InternalRouteLink } from 'react-router-dom';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredIamRoleName } from 'shared/components/Validation/rules';
import styled from 'styled-components';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import {
  ShowConfigurationScript,
  RoleArnInput,
} from 'teleport/Integrations/shared';
import { AWS_RESOURCE_GROUPS_TAG_EDITOR_LINK } from 'teleport/Discover/Shared/const';
import useStickyClusterId from 'teleport/useStickyClusterId';
import { AwsOidcPolicyPreset } from 'teleport/services/integrations';

import { FinishDialog } from './FinishDialog';
import { useAwsOidcIntegration } from './useAwsOidcIntegration';

export function AwsOidc() {
  const {
    integrationConfig,
    setIntegrationConfig,
    scriptUrl,
    setScriptUrl,
    handleOnCreate,
    createdIntegration,
    createIntegrationAttempt,
    generateAwsOidcConfigIdpScript,
  } = useAwsOidcIntegration();
  const { clusterId } = useStickyClusterId();

  return (
    <Box pt={3}>
      <Header>Set up your AWS account</Header>

      <Box width="800px" mb={4}>
        Instead of storing long-lived static credentials, Teleport will become a
        trusted OIDC provider with AWS to be able to request short lived
        credentials when performing operations automatically such as when
        connecting{' '}
        <RouteLink
          to={{
            pathname: `${cfg.routes.root}/discover`,
            state: { searchKeywords: 'ec2' },
          }}
        >
          AWS EC2
        </RouteLink>{' '}
        or{' '}
        <RouteLink
          to={{
            pathname: `${cfg.routes.root}/discover`,
            state: { searchKeywords: 'rds' },
          }}
        >
          AWS RDS
        </RouteLink>{' '}
        instances during resource enrollment.
        <Box mt={3}>
          AWS Resources created by the integration are tagged so that you can
          search and export them using the{' '}
          <Link target="_blank" href={AWS_RESOURCE_GROUPS_TAG_EDITOR_LINK}>
            AWS Resource Groups / Tag Editor
          </Link>
          . The following tags are applied:
          <TextSelectCopyMulti
            bash={false}
            lines={[
              {
                text:
                  `teleport.dev/cluster: ` +
                  clusterId +
                  `\n` +
                  `teleport.dev/origin: integration_awsoidc\n` +
                  `teleport.dev/integration: ` +
                  integrationConfig.name,
              },
            ]}
          />
        </Box>
      </Box>

      <Validation>
        {({ validator }) => (
          <>
            <Container mb={5} width={800}>
              <Text bold>Step 1</Text>
              <Box width="600px">
                <FieldInput
                  autoFocus={true}
                  value={integrationConfig.name}
                  label="Give this AWS integration a name"
                  placeholder="Integration Name"
                  onChange={e =>
                    setIntegrationConfig({
                      ...integrationConfig,
                      name: e.target.value,
                    })
                  }
                  disabled={!!scriptUrl}
                />
                <FieldInput
                  rule={requiredIamRoleName}
                  value={integrationConfig.roleName}
                  placeholder="Integration role name"
                  label="Give a name for an AWS IAM role this integration will create"
                  onChange={e =>
                    setIntegrationConfig({
                      ...integrationConfig,
                      roleName: e.target.value,
                    })
                  }
                  disabled={!!scriptUrl}
                />
              </Box>
              {scriptUrl ? (
                <ButtonSecondary
                  mb={3}
                  onClick={() => {
                    setScriptUrl('');
                  }}
                >
                  Edit
                </ButtonSecondary>
              ) : (
                <ButtonSecondary
                  mb={3}
                  onClick={() =>
                    generateAwsOidcConfigIdpScript(
                      validator,
                      AwsOidcPolicyPreset.Unspecified
                    )
                  }
                >
                  Generate Command
                </ButtonSecondary>
              )}
            </Container>
            {scriptUrl && (
              <>
                <Container mb={5} width={800}>
                  <Text bold>Step 2</Text>
                  <ShowConfigurationScript scriptUrl={scriptUrl} />
                </Container>
                <Container mb={5} width={800}>
                  <Text bold>Step 3</Text>
                  <RoleArnInput
                    roleName={integrationConfig.roleName}
                    roleArn={integrationConfig.roleArn}
                    setRoleArn={(v: string) =>
                      setIntegrationConfig({
                        ...integrationConfig,
                        roleArn: v,
                      })
                    }
                    disabled={createIntegrationAttempt.status === 'processing'}
                  />
                </Container>
              </>
            )}
            {createIntegrationAttempt.status === 'error' && (
              <Flex>
                <Icons.Warning mr={2} color="error.main" size="small" />
                <Text color="error.main">
                  Error: {createIntegrationAttempt.statusText}
                </Text>
              </Flex>
            )}
            <Box mt={6}>
              <ButtonPrimary
                onClick={() => handleOnCreate(validator)}
                disabled={
                  !scriptUrl ||
                  createIntegrationAttempt.status === 'processing' ||
                  !integrationConfig.roleArn
                }
              >
                Create Integration
              </ButtonPrimary>
              <ButtonSecondary
                ml={3}
                as={InternalRouteLink}
                to={cfg.getIntegrationEnrollRoute(null)}
              >
                Back
              </ButtonSecondary>
            </Box>
          </>
        )}
      </Validation>
      {createdIntegration && <FinishDialog integration={createdIntegration} />}
    </Box>
  );
}

export function AwdOidcConfigureName({
  integrationConfig,
  setIntegrationConfig,
  disabled,
}: {
  integrationConfig: integrationConfig;
  setIntegrationConfig: (ic: integrationConfig) => void;
  disabled: boolean;
}) {
  return (
    <Box width="500px" mt={2}>
      <FieldInput
        rule={requiredIamRoleName}
        autoFocus={true}
        value={integrationConfig.name}
        label="Give this AWS integration a name"
        placeholder="Integration Name"
        onChange={e =>
          setIntegrationConfig({
            ...integrationConfig,
            name: e.target.value,
          })
        }
        width="500px"
        disabled={disabled}
      />
      {/* {showRoleNameInput && <FieldInput
      rule={requiredIamRoleName}
      value={integrationConfig.roleArn}
      placeholder="IAM Role Name"
      label="Give a name to an IAM role that this AWS integration will create"
      onChange={e => setIntegrationRequest({...integrationRequest, awsoidc:{ roleArn: e.target.value}})}
      disabled={!!scriptUrl}
    />} */}
    </Box>
  );
}

export function AwsOidcShowCommand({
  scriptUrl,
  description,
}: {
  scriptUrl: string;
  description?: React.ReactNode;
}) {
  return (
    <Box>
      {description || (
        <Text>
          Open{' '}
          <Link
            href="https://console.aws.amazon.com/cloudshell/home"
            target="_blank"
          >
            AWS CloudShell
          </Link>{' '}
          and copy and paste the command that configures the permissions for you
        </Text>
      )}
      <Box mb={2} mt={3}>
        <TextSelectCopyMulti
          lines={[
            {
              text: `bash -c "$(curl '${scriptUrl}')"`,
            },
          ]}
        />
      </Box>
    </Box>
  );
}

export function AwsOidcConfigureRoleArn({
  description,
  integrationConfig,
  setIntegrationConfig,
  disabled,
}: {
  description?: React.ReactNode;
  integrationConfig: integrationConfig;
  setIntegrationConfig: (ic: integrationConfig) => void;
  disabled: boolean;
}) {
  return (
    <Box>
      {description || (
        <Text>
          After configuring is finished, go to your{' '}
          <Link
            target="_blank"
            href={`https://console.aws.amazon.com/iamv2/home#/roles/details/${integrationConfig.name}`}
          >
            IAM Role dashboard
          </Link>{' '}
          and copy and paste the ARN below.
        </Text>
      )}
      <FieldInput
        mt={3}
        rule={requiredRoleArn(integrationConfig.name)}
        value={integrationConfig.roleArn}
        label="Role ARN (Amazon Resource Name)"
        placeholder={`arn:aws:iam::123456789012:role/${integrationConfig.name}`}
        width="500px"
        onChange={e =>
          setIntegrationConfig({
            ...integrationConfig,
            roleArn: e.target.value,
          })
        }
        // onChange={e => setRoleArn(e.target.value)}
        disabled={disabled}
        toolTipContent={`Unique AWS resource identifier and uses the format: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`}
      />
    </Box>
  );
}

export function AwsOidc({
  header,
  subHeader,
}: {
  header?: string;
  subHeader?: React.ReactNode;
}) {
  return (
    <Box pt={3}>
      <Header>{header || 'Set up your AWS account'}</Header>

      {subHeader || (
        <Box width="800px" mb={4}>
          Instead of storing long-lived static credentials, Teleport will become
          a trusted OIDC provider with AWS to be able to request short lived
          credentials when performing operations automatically such as when
          connecting{' '}
          <RouteLink
            to={{
              pathname: `${cfg.routes.root}/discover`,
              state: { searchKeywords: 'ec2' },
            }}
          >
            AWS EC2
          </RouteLink>{' '}
          or{' '}
          <RouteLink
            to={{
              pathname: `${cfg.routes.root}/discover`,
              state: { searchKeywords: 'rds' },
            }}
          >
            AWS RDS
          </RouteLink>{' '}
          instances during resource enrollment.
        </Box>
      )}

      <AwsOidcIntegrationScriptGenerator />
    </Box>
  );
}

const Container = styled(Box)`
  max-width: 1000px;
  background-color: ${p => p.theme.colors.spotBackground[0]};
  border-radius: ${p => `${p.theme.space[2]}px`};
  padding: ${p => p.theme.space[3]}px;
`;

const RouteLink = styled(InternalRouteLink)`
  color: ${({ theme }) => theme.colors.buttons.link.default};

  &:hover,
  &:focus {
    color: ${({ theme }) => theme.colors.buttons.link.hover};
  }
`;
