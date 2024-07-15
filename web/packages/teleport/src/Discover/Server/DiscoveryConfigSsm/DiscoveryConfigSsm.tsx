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

import React, { useState, useRef } from 'react';
import {
  Box,
  Link as ExternalLink,
  Text,
  Flex,
  ButtonSecondary,
  Mark,
} from 'design';
import styled from 'styled-components';
import { Danger, Info } from 'design/Alert';
import TextEditor from 'shared/components/TextEditor';
import { ToolTipInfo } from 'shared/components/ToolTip';
import FieldInput from 'shared/components/FieldInput';
import { Rule } from 'shared/components/Validation/rules';
import Validation, { Validator } from 'shared/components/Validation';
import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { Regions } from 'teleport/services/integrations';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import JoinTokenService, { JoinToken } from 'teleport/services/joinToken';

import {
  DISCOVERY_GROUP_CLOUD,
  createDiscoveryConfig,
  InstallParamEnrollMode,
} from 'teleport/services/discovery';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { ActionButtons, Header, StyledBox } from '../../Shared';

import { SingleEc2InstanceInstallation } from '../Shared';

import { DiscoveryConfigCreatedDialog } from './DiscoveryConfigCreatedDialog';

const IAM_POLICY_NAME = 'EC2DiscoverWithSSM';

export function DiscoveryConfigSsm() {
  const { agentMeta, emitErrorEvent, nextStep, updateAgentMeta, prevStep } =
    useDiscover();

  const { arnResourceName, awsAccountId } = splitAwsIamArn(
    agentMeta.awsIntegration.spec.roleArn
  );

  const { clusterId } = useStickyClusterId();

  const [selectedRegion, setSelectedRegion] = useState<Regions>();
  const [ssmDocumentName, setSsmDocumentName] = useState(
    'TeleportDiscoveryInstaller'
  );
  const [scriptUrl, setScriptUrl] = useState('');
  const joinTokenRef = useRef<JoinToken>();
  const [showRestOfSteps, setShowRestOfSteps] = useState(false);

  const [attempt, createJoinTokenAndDiscoveryConfig, setAttempt] = useAsync(
    async () => {
      try {
        const joinTokenService = new JoinTokenService();
        // Don't create another token if token was already created.
        // This can happen if creating discovery config attempt failed
        // and the user retries.
        if (!joinTokenRef.current) {
          joinTokenRef.current = await joinTokenService.fetchJoinToken({
            roles: ['Node'],
            method: 'iam',
            rules: [{ awsAccountId }],
          });
        }

        const config = await createDiscoveryConfig(clusterId, {
          name: crypto.randomUUID(),
          discoveryGroup: cfg.isCloud
            ? DISCOVERY_GROUP_CLOUD
            : agentMeta.autoDiscovery.config.discoveryGroup,
          aws: [
            {
              types: ['ec2'],
              regions: [selectedRegion],
              tags: { '*': ['*'] },
              integration: agentMeta.awsIntegration.name,
              ssm: { documentName: ssmDocumentName },
              install: {
                enrollMode: InstallParamEnrollMode.Script,
                installTeleport: true,
                joinToken: joinTokenRef.current.id,
              },
            },
          ],
        });

        updateAgentMeta({
          ...agentMeta,
          awsRegion: selectedRegion,
          autoDiscovery: {
            config,
          },
        });
      } catch (err) {
        emitErrorEvent(err.message);
        throw err;
      }
    }
  );

  function generateScriptUrl(validator: Validator) {
    if (!validator.validate()) return;

    const scriptUrl = cfg.getAwsIamConfigureScriptEc2AutoDiscoverWithSsmUrl({
      iamRoleName: arnResourceName,
      region: selectedRegion,
      ssmDocument: ssmDocumentName,
    });
    setScriptUrl(scriptUrl);
  }

  function clear() {
    setAttempt(makeEmptyAttempt);
    joinTokenRef.current = undefined;
  }

  function handleOnSubmit(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (scriptUrl) {
      setScriptUrl('');
      return;
    }
    generateScriptUrl(validator);
  }

  return (
    <Box maxWidth="1000px">
      <Header>Setup Discovery Config for Teleport Discovery Service</Header>
      {cfg.isCloud ? (
        <Text>
          The Teleport Discovery Service can connect to Amazon EC2 and
          automatically discover and enroll EC2 instances. <SsmInfoHeaderText />
        </Text>
      ) : (
        <Text>
          Discovery config defines the setup that enables Teleport to
          automatically discover and register instances. <SsmInfoHeaderText />
        </Text>
      )}
      {cfg.isCloud && <SingleEc2InstanceInstallation />}
      {attempt.status === 'error' && (
        <Danger mt={3}>{attempt.statusText}</Danger>
      )}
      <StyledBox mt={4}>
        <Text bold>Step 1</Text>
        <Box mb={-5}>
          <Text typography="subtitle1">
            Select the AWS Region that contains the EC2 instances that you would
            like to enroll:
          </Text>
          <AwsRegionSelector
            onFetch={(region: Regions) => setSelectedRegion(region)}
            clear={clear}
            disableSelector={!!scriptUrl}
          />
        </Box>
        {!showRestOfSteps && (
          <ButtonSecondary
            onClick={() => setShowRestOfSteps(true)}
            disabled={!selectedRegion}
            mt={3}
          >
            Next
          </ButtonSecondary>
        )}
        {scriptUrl && (
          <ButtonSecondary onClick={() => setScriptUrl('')} mt={3}>
            Edit
          </ButtonSecondary>
        )}
      </StyledBox>
      {showRestOfSteps && (
        <>
          <StyledBox mt={4}>
            <Text bold>Step 2</Text>
            <Text typography="subtitle1">
              Attach AWS managed{' '}
              <ExternalLink
                target="_blank"
                href="https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html"
              >
                AmazonSSMManagedInstanceCore
              </ExternalLink>{' '}
              policy to EC2 instances IAM profile. The policy enables EC2
              instances to use SSM core functionality.
            </Text>
          </StyledBox>
          <StyledBox mt={4}>
            <Text bold>Step 3</Text>
            Each EC2 instance requires{' '}
            <ExternalLink
              target="_blank"
              href="https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent-status-and-restart.html"
            >
              SSM Agent
            </ExternalLink>{' '}
            to be running. The SSM{' '}
            <ExternalLink
              target="_blank"
              href={`https://${selectedRegion}.console.aws.amazon.com/systems-manager/fleet-manager/managed-nodes?region=${selectedRegion}`}
            >
              Nodes Manager dashboard
            </ExternalLink>{' '}
            will list all instances that have SSM agent already running. Ensure
            ping statuses are <Mark>Online</Mark>.
            <Info mt={3} mb={0}>
              If you do not see your instances listed in the dashboard, it might
              take up to 30 minutes for your instances to use the IAM
              credentials you updated in step 2.
            </Info>
          </StyledBox>
          <Validation>
            {({ validator }) => (
              <form>
                <StyledBox mt={4}>
                  <Text bold>Step 4</Text>
                  <Box>
                    <Text typography="subtitle1" mb={1}>
                      Give a name for the{' '}
                      <ExternalLink
                        target="_blank"
                        href="https://docs.aws.amazon.com/systems-manager/latest/userguide/documents.html"
                      >
                        AWS SSM Document
                      </ExternalLink>{' '}
                      that will be created on your behalf. Required to run the
                      installer script on each discovered instances.
                    </Text>
                    <FieldInput
                      rule={requiredSsmDocument}
                      label="SSM Document Name"
                      value={ssmDocumentName}
                      onChange={e => setSsmDocumentName(e.target.value)}
                      placeholder="ssm-document-name"
                      disabled={!!scriptUrl}
                    />
                  </Box>
                  <ButtonSecondary
                    type="submit"
                    onClick={e => handleOnSubmit(e, validator)}
                    disabled={!selectedRegion}
                  >
                    {scriptUrl ? 'Edit' : 'Next'}
                  </ButtonSecondary>
                </StyledBox>
              </form>
            )}
          </Validation>
          {scriptUrl && (
            <StyledBox mt={4}>
              <Text bold>Step 5</Text>
              <Flex alignItems="center" gap={1} mb={2}>
                <Text typography="subtitle1">
                  Run the command below on your{' '}
                  <ExternalLink
                    href="https://console.aws.amazon.com/cloudshell/home"
                    target="_blank"
                  >
                    AWS CloudShell
                  </ExternalLink>{' '}
                  to configure your IAM permissions.
                </Text>
                <ToolTipInfo sticky={true} maxWidth={450}>
                  The following IAM permissions will be added as an inline
                  policy named <Mark>{IAM_POLICY_NAME}</Mark> to IAM role{' '}
                  <Mark>{arnResourceName}</Mark>
                  <Box mb={2}>
                    <EditorWrapper $height={350}>
                      <TextEditor
                        readOnly={true}
                        data={[{ content: inlinePolicyJson, type: 'json' }]}
                        bg="levels.deep"
                      />
                    </EditorWrapper>
                  </Box>
                </ToolTipInfo>
              </Flex>
              <TextSelectCopyMulti
                lines={[{ text: `bash -c "$(curl '${scriptUrl}')"` }]}
              />
            </StyledBox>
          )}
        </>
      )}

      {attempt.status === 'success' && (
        <DiscoveryConfigCreatedDialog toNextStep={nextStep} />
      )}

      <ActionButtons
        onProceed={createJoinTokenAndDiscoveryConfig}
        onPrev={prevStep}
        disableProceed={attempt.status === 'processing' || !scriptUrl}
      />
    </Box>
  );
}

const EditorWrapper = styled(Flex)<{ $height: number }>`
  flex-directions: column;
  height: ${p => p.$height}px;
  margin-top: ${p => p.theme.space[3]}px;
  width: 450px;
`;

const inlinePolicyJson = `{
  "Version": "2012-10-17",
  "Statement": [
      {
          "Effect": "Allow",
          "Action": [
              "ec2:DescribeInstances",
              "ssm:DescribeInstanceInformation",
              "ssm:GetCommandInvocation",
              "ssm:ListCommandInvocations",
              "ssm:SendCommand"
          ],
          "Resource": "*"
      }
  ]
}`;

const SSM_DOCUMENT_NAME_REGEX = new RegExp('^[0-9A-Za-z._-]*$');
const requiredSsmDocument: Rule = name => () => {
  if (!name || name.length < 3 || name.length > 128) {
    return {
      valid: false,
      message: 'name must be between 3 and 128 characters',
    };
  }

  const match = name.match(SSM_DOCUMENT_NAME_REGEX);
  if (!match) {
    return {
      valid: false,
      message: 'valid characters are a-z, A-Z, 0-9, and _, -, and . only',
    };
  }

  return {
    valid: true,
  };
};

const SsmInfoHeaderText = () => (
  <>
    The service will execute an install script on these discovered instances
    using{' '}
    <ExternalLink
      target="_blank"
      href="https://docs.aws.amazon.com/systems-manager/latest/userguide/what-is-systems-manager.html"
    >
      AWS Systems Manager
    </ExternalLink>{' '}
    that will install Teleport, start it and join the cluster.
  </>
);
