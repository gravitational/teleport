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

import React, { useEffect, useState } from 'react';
import { Link as InternalRouteLink } from 'react-router-dom';
import { useLocation } from 'react-router';
import styled from 'styled-components';
import { Box, ButtonSecondary, Text, Link, Flex, ButtonPrimary } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { requiredIamRoleName } from 'shared/components/Validation/rules';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  IntegrationEnrollEvent,
  IntegrationEnrollEventData,
  IntegrationEnrollKind,
  userEventService,
} from 'teleport/services/userEvent';
import { Header } from 'teleport/Discover/Shared';
import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import {
  Integration,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';

import { FinishDialog } from './FinishDialog';

type integrationConfig = {
  name: string;
  roleArn: string;
};

export function useAwsOidcIntegration() {
  const [integrationConfig, setIntegrationConfig] = useState<integrationConfig>(
    {
      name: '',
      roleArn: '',
    }
  );
  const [scriptUrl, setScriptUrl] = useState('');
  const [createdIntegration, setCreatedIntegration] = useState<Integration>();
  const { attempt, run } = useAttempt('');

  const location = useLocation<DiscoverUrlLocationState>();

  const [eventData] = useState<IntegrationEnrollEventData>({
    id: crypto.randomUUID(),
    kind: IntegrationEnrollKind.AwsOidc,
  });

  useEffect(() => {
    // If a user came from the discover wizard,
    // discover wizard will send of appropriate events.
    if (location.state?.discover) {
      return;
    }

    emitEvent(IntegrationEnrollEvent.Started);
    // Only send event once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function emitEvent(event: IntegrationEnrollEvent) {
    userEventService.captureIntegrationEnrollEvent({
      event,
      eventData,
    });
  }

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      integrationService
        .createIntegration({
          name: integrationConfig.name,
          subKind: IntegrationKind.AwsOidc,
          awsoidc: {
            roleArn: integrationConfig.roleArn,
          },
        })
        // .createIntegration(integrationRequest)
        .then(res => {
          setCreatedIntegration(res);

          if (location.state?.discover) {
            return;
          }
          emitEvent(IntegrationEnrollEvent.Complete);
        })
    );
  }

  function generateAwsOidcConfigIdpScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    const newScriptUrl = cfg.getAwsOidcConfigureIdpScriptUrl({
      integrationName: integrationConfig.name,
      roleName: integrationConfig.name,
    });

    setScriptUrl(newScriptUrl);
  }

  return {
    integrationConfig,
    setIntegrationConfig,
    scriptUrl,
    setScriptUrl,
    createdIntegration,
    handleOnCreate,
    generateAwsOidcConfigIdpScript,
    attempt,
  };
}

export function AwsOidcIntegrationScriptGenerator({
  createIntegration = true,
}: {
  createIntegration?: boolean;
}) {
  const {
    integrationConfig,
    setIntegrationConfig,
    scriptUrl,
    setScriptUrl,
    createdIntegration,
    handleOnCreate,
    attempt,
    generateAwsOidcConfigIdpScript,
  } = useAwsOidcIntegration();

  return (
    <Box>
      <Validation>
        {({ validator }) => (
          <>
            <Container mb={5}>
              <Text bold>Step 1: Configure AWS OIDC integraiton name</Text>
              <Text>
                The name will be used to as integraiton name and AWS IAM role
                name.{' '}
              </Text>
              <AwdOidcConfigureName
                integrationConfig={integrationConfig}
                setIntegrationConfig={setIntegrationConfig}
                disabled={!!scriptUrl}
              />
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
                  onClick={() => generateAwsOidcConfigIdpScript(validator)}
                >
                  Generate Command
                </ButtonSecondary>
              )}
            </Container>
            {scriptUrl && (
              <>
                <Container mb={5}>
                  <Text bold>Step 2</Text>
                  <AwsOidcShowCommand scriptUrl={scriptUrl} />
                </Container>
                <Container mb={5}>
                  <Text bold>Step 3</Text>
                  <AwsOidcConfigureRoleArn
                    integrationConfig={integrationConfig}
                    setIntegrationConfig={setIntegrationConfig}
                    disabled={attempt.status === 'processing'}
                  />
                </Container>
              </>
            )}
            {attempt.status === 'failed' && (
              <Flex>
                <Icons.Warning mr={2} color="error.main" size="small" />
                <Text color="error.main">Error: {attempt.statusText}</Text>
              </Flex>
            )}
            {createIntegration && (
              <Box mt={6}>
                <ButtonPrimary
                  onClick={() => handleOnCreate(validator)}
                  disabled={
                    !scriptUrl ||
                    attempt.status === 'processing' ||
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
            )}
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

const requiredRoleArn = (roleName: string) => (roleArn: string) => () => {
  const regex = new RegExp(
    '^arn:aws.*:iam::\\d{12}:role\\/(' + roleName + ')$'
  );

  if (regex.test(roleArn)) {
    return {
      valid: true,
    };
  }

  return {
    valid: false,
    message:
      'invalid role ARN, double check you copied and pasted the correct output',
  };
};

const RouteLink = styled(InternalRouteLink)`
  color: ${({ theme }) => theme.colors.buttons.link.default};

  &:hover,
  &:focus {
    color: ${({ theme }) => theme.colors.buttons.link.hover};
  }
`;
