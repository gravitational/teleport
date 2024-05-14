/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React from 'react';
import styled from 'styled-components';
import { Box, Text, Flex, Link } from 'design';
import TextEditor from 'shared/components/TextEditor';
import { Danger } from 'design/Alert';
import useAttempt from 'shared/hooks/useAttemptNext';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { integrationService } from 'teleport/services/integrations';
import cfg from 'teleport/config';
import { Container } from 'teleport/Discover/Shared/CommandBox';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';

import { ActionButtons, Header, Mark } from '../../Shared';

import { CreatedDialog } from './CreatedDialog';

const IAM_POLICY_NAME = 'TeleportAWSAccess';

export function CreateAppAccess() {
  const { agentMeta, updateAgentMeta, emitErrorEvent, nextStep } =
    useDiscover();
  const { awsIntegration } = agentMeta;
  const { attempt, setAttempt } = useAttempt('');

  function handleOnProceed() {
    setAttempt({ status: 'processing' });

    integrationService
      .createAwsAppAccess(awsIntegration.name)
      .then(app => {
        updateAgentMeta({
          ...agentMeta,
          app,
          awsRoleArns: app.awsRoles.map(r => r.arn),
          resourceName: app.name,
        });
        setAttempt({ status: 'success' });
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
        emitErrorEvent(err.message);
      });
  }

  const iamRoleName = splitAwsIamArn(
    agentMeta.awsIntegration.spec.roleArn
  ).arnResourceName;
  const scriptUrl = cfg.getAwsIamConfigureScriptAppAccessUrl({
    iamRoleName,
  });

  return (
    <Box maxWidth="800px">
      <Header>Create AWS Application Access</Header>
      <Text mt={1} mb={3}>
        An application server will be created that will use the AWS OIDC
        Integration <Mark>{agentMeta.awsIntegration.name}</Mark> for proxying
        access.
      </Text>
      {attempt.status === 'failed' && (
        <Danger mt={3}>{attempt.statusText}</Danger>
      )}
      <Container>
        <Flex alignItems="center" gap={1} mb={1}>
          <Text bold>First configure your AWS IAM permissions</Text>
          <ToolTipInfo sticky={true} maxWidth={450}>
            The following IAM permissions will be added as an inline policy
            named <Mark>{IAM_POLICY_NAME}</Mark> to IAM role{' '}
            <Mark>{iamRoleName}</Mark>
            <Box mb={2}>
              <EditorWrapper $height={250}>
                <TextEditor
                  readOnly={true}
                  data={[{ content: inlinePolicyJson, type: 'json' }]}
                  bg="levels.deep"
                />
              </EditorWrapper>
            </Box>
          </ToolTipInfo>
        </Flex>
        <Text typography="subtitle1" mb={1}>
          Run the command below on your{' '}
          <Link
            href="https://console.aws.amazon.com/cloudshell/home"
            target="_blank"
          >
            AWS CloudShell
          </Link>{' '}
          to configure your IAM permissions.
        </Text>
        <TextSelectCopyMulti
          lines={[{ text: `bash -c "$(curl '${scriptUrl}')"` }]}
        />
      </Container>

      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={
          attempt.status === 'processing' || attempt.status === 'success'
        }
      />
      {attempt.status === 'success' && (
        <CreatedDialog nextStep={nextStep} appName={agentMeta.resourceName} />
      )}
    </Box>
  );
}

const EditorWrapper = styled(Flex)`
  flex-directions: column;
  height: ${p => p.$height}px;
  margin-top: ${p => p.theme.space[3]}px;
  width: 450px;
`;

const inlinePolicyJson = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AssumeTaggedRole",
      "Effect": "Allow",
      "Action": "sts:AssumeRole",
      "Resource": "*",
      "Condition": {
        "StringEquals": {"iam:ResourceTag/teleport.dev/integration": "true"}
      }
    }
  ]
}`;
