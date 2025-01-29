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

import { Link as InternalRouteLink } from 'react-router-dom';
import styled from 'styled-components';

import { Box, ButtonPrimary, ButtonSecondary, Flex, Link, Text } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation from 'shared/components/Validation';
import { requiredIamRoleName } from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import { Header } from 'teleport/Discover/Shared';
import { AWS_RESOURCE_GROUPS_TAG_EDITOR_LINK } from 'teleport/Discover/Shared/const';
import {
  RoleArnInput,
  ShowConfigurationScript,
} from 'teleport/Integrations/shared';
import { AwsOidcPolicyPreset } from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { ConfigureAwsOidcSummary } from './ConfigureAwsOidcSummary';
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
                  <Flex gap={1} alignItems="center">
                    <Text bold>Step 2</Text>
                    <ConfigureAwsOidcSummary
                      roleName={integrationConfig.roleName}
                      integrationName={integrationConfig.name}
                    />
                  </Flex>
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
