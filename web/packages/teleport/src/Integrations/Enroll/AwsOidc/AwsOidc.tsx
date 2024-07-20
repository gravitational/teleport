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

export function AwsOidc() {
  const [integrationName, setIntegrationName] = useState('');
  const [roleArn, setRoleArn] = useState('');
  const [roleName, setRoleName] = useState('');
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

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() =>
      integrationService
        .createIntegration({
          name: integrationName,
          subKind: IntegrationKind.AwsOidc,
          awsoidc: {
            roleArn,
          },
        })
        .then(res => {
          setCreatedIntegration(res);

          if (location.state?.discover) {
            return;
          }
          emitEvent(IntegrationEnrollEvent.Complete);
        })
    );
  }

  function emitEvent(event: IntegrationEnrollEvent) {
    userEventService.captureIntegrationEnrollEvent({
      event,
      eventData,
    });
  }

  function generateAwsOidcConfigIdpScript(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    const newScriptUrl = cfg.getAwsOidcConfigureIdpScriptUrl({
      integrationName,
      roleName,
    });

    setScriptUrl(newScriptUrl);
  }

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
      </Box>

      <Validation>
        {({ validator }) => (
          <>
            <Container mb={5}>
              <Text bold>Step 1</Text>
              <Box width="600px">
                <FieldInput
                  autoFocus={true}
                  value={integrationName}
                  label="Give this AWS integration a name"
                  placeholder="Integration Name"
                  onChange={e => setIntegrationName(e.target.value)}
                  disabled={!!scriptUrl}
                />
                <FieldInput
                  rule={requiredIamRoleName}
                  value={roleName}
                  placeholder="IAM Role Name"
                  label="IAM Role Name"
                  onChange={e => setRoleName(e.target.value)}
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
                  <Text mb={2}>
                    Open{' '}
                    <Link
                      href="https://console.aws.amazon.com/cloudshell/home"
                      target="_blank"
                    >
                      AWS CloudShell
                    </Link>{' '}
                    and copy and paste the command that configures the
                    permissions for you:
                  </Text>
                  <Box mb={2}>
                    <TextSelectCopyMulti
                      lines={[
                        {
                          text: `bash -c "$(curl '${scriptUrl}')"`,
                        },
                      ]}
                    />
                  </Box>
                </Container>
                <Container mb={5}>
                  <Text bold>Step 3</Text>
                  After configuring is finished, go to your{' '}
                  <Link
                    target="_blank"
                    href={`https://console.aws.amazon.com/iamv2/home#/roles/details/${roleName}`}
                  >
                    IAM Role dashboard
                  </Link>{' '}
                  and copy and paste the ARN below.
                  <FieldInput
                    mt={2}
                    rule={requiredRoleArn(roleName)}
                    value={roleArn}
                    label="Role ARN (Amazon Resource Name)"
                    placeholder={`arn:aws:iam::123456789012:role/${roleName}`}
                    width="430px"
                    onChange={e => setRoleArn(e.target.value)}
                    disabled={attempt.status === 'processing'}
                    toolTipContent={`Unique AWS resource identifier and uses the format: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`}
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
            <Box mt={6}>
              <ButtonPrimary
                onClick={() => handleOnCreate(validator)}
                disabled={
                  !scriptUrl || attempt.status === 'processing' || !roleArn
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
