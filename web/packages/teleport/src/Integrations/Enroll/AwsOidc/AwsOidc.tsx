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
import {
  Box,
  ButtonSecondary,
  Text,
  Link,
  Flex,
  ButtonPrimary,
  ButtonText,
} from 'design';
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
import { S3BucketConfiguration } from './S3BucketConfiguration';
import {
  getDefaultS3BucketName,
  requiredPrefixName,
  validPrefixNameToolTipContent,
} from './Shared/utils';
import { S3BucketWarningBanner } from './S3BucketWarningBanner';

export function AwsOidc() {
  const [integrationName, setIntegrationName] = useState('');
  const [roleArn, setRoleArn] = useState('');
  const [roleName, setRoleName] = useState('');
  const [scriptUrl, setScriptUrl] = useState('');
  const [s3Bucket, setS3Bucket] = useState(() => getDefaultS3BucketName());
  const [s3Prefix, setS3Prefix] = useState('');
  const [showS3BucketWarning, setShowS3BucketWarning] = useState(false);
  const [confirmedS3BucketWarning, setConfirmedS3BucketWarning] =
    useState(false);
  const [createdIntegration, setCreatedIntegration] = useState<Integration>();
  const { attempt, run } = useAttempt('');

  const location = useLocation<DiscoverUrlLocationState>();

  const [eventData] = useState<IntegrationEnrollEventData>({
    id: crypto.randomUUID(),
    kind: IntegrationEnrollKind.AwsOidc,
  });

  const requiresS3BucketWarning = !s3Bucket && !s3Prefix;

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
            issuerS3Bucket: s3Bucket,
            issuerS3Prefix: s3Prefix,
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
      s3Bucket,
      s3Prefix,
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
                  rule={requiredPrefixName(true)}
                  autoFocus={true}
                  value={integrationName}
                  label="Give this AWS integration a name"
                  placeholder="Integration Name"
                  onChange={e => setIntegrationName(e.target.value)}
                  disabled={!!scriptUrl}
                  onBlur={() => {
                    // s3Bucket by default is defined.
                    // If empty user intentionally cleared it.
                    if (!integrationName || (!s3Bucket && !s3Prefix)) return;
                    // Help come up with a default prefix name for user.
                    if (!s3Prefix) {
                      setS3Prefix(`${integrationName}-oidc-idp`);
                    }
                  }}
                  toolTipContent={validPrefixNameToolTipContent('Integration')}
                />
                <FieldInput
                  rule={requiredIamRoleName}
                  value={roleName}
                  placeholder="IAM Role Name"
                  label="IAM Role Name"
                  onChange={e => setRoleName(e.target.value)}
                  disabled={!!scriptUrl}
                />
                <S3BucketConfiguration
                  s3Bucket={s3Bucket}
                  setS3Bucket={setS3Bucket}
                  s3Prefix={s3Prefix}
                  setS3Prefix={setS3Prefix}
                  disabled={!!scriptUrl}
                />
              </Box>
              {confirmedS3BucketWarning && (
                <Box>
                  <ButtonText
                    pl={0}
                    gap={2}
                    onClick={() => setShowS3BucketWarning(true)}
                    alignItems="center"
                  >
                    <Icons.Warning size="small" color="warning.main" />
                    <Text fontSize={1}>Click to view S3 Bucket Warning</Text>
                  </ButtonText>
                </Box>
              )}
              {showS3BucketWarning ? (
                <S3BucketWarningBanner
                  onClose={() => setShowS3BucketWarning(false)}
                  onContinue={() => {
                    setShowS3BucketWarning(false);
                    setConfirmedS3BucketWarning(true);
                    generateAwsOidcConfigIdpScript(validator);
                  }}
                  reviewing={confirmedS3BucketWarning}
                />
              ) : scriptUrl ? (
                <ButtonSecondary
                  mb={3}
                  onClick={() => {
                    setScriptUrl('');
                    setConfirmedS3BucketWarning(false);
                  }}
                >
                  Edit
                </ButtonSecondary>
              ) : (
                <ButtonSecondary
                  mb={3}
                  onClick={() => {
                    if (requiresS3BucketWarning) {
                      setShowS3BucketWarning(true);
                    } else {
                      generateAwsOidcConfigIdpScript(validator);
                    }
                  }}
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
