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

import React, { useRef, useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonSecondary,
  Link as ExternalLink,
  Flex,
  H3,
  Mark,
  Subtitle3,
  Text,
} from 'design';
import { Danger, Info } from 'design/Alert';
import { P } from 'design/Text/Text';
import { IconTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import TextEditor from 'shared/components/TextEditor';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { Validator } from 'shared/components/Validation';
import { Rule } from 'shared/components/Validation/rules';
import { makeEmptyAttempt, useAsync } from 'shared/hooks/useAsync';

import { LabelsInput } from 'teleport/components/LabelsInput';
import cfg from 'teleport/config';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  createDiscoveryConfig,
  DISCOVERY_GROUP_CLOUD,
  InstallParamEnrollMode,
  Labels,
} from 'teleport/services/discovery';
import { Regions } from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import JoinTokenService, { JoinToken } from 'teleport/services/joinToken';
import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { ActionButtons, Header, StyledBox } from '../../Shared';
import { SingleEc2InstanceInstallation } from '../Shared';
import { DiscoveryConfigCreatedDialog } from './DiscoveryConfigCreatedDialog';

const IAM_POLICY_NAME = 'EC2DiscoverWithSSM';

type AWSLabel = {
  name: string;
  value: string;
};

type AWSLabels = AWSLabel[];

function makeTags(labels: AWSLabels): Labels {
  if (labels.length === 0) {
    return { '*': ['*'] };
  }
  const output: Labels = {};

  labels.forEach(label => {
    if (label.name === '' || label.value === '') {
      return;
    }

    if (output[label.name]) {
      const labelSet = new Set(output[label.name]);
      labelSet.add(label.value);
      output[label.name] = Array.from(labelSet);
      return;
    }
    output[label.name] = [label.value];
  });
  return output;
}

export function DiscoveryConfigSsm() {
  const {
    agentMeta,
    emitErrorEvent,
    nextStep,
    updateAgentMeta,
    prevStep,
    emitEvent,
  } = useDiscover();

  const { arnResourceName, awsAccountId } = splitAwsIamArn(
    agentMeta.awsIntegration.spec.roleArn
  );

  const { clusterId } = useStickyClusterId();

  const [selectedRegion, setSelectedRegion] = useState<Regions>();
  const [ssmDocumentName, setSsmDocumentName] = useState(
    'TeleportDiscoveryInstaller'
  );
  const [scriptUrl, setScriptUrl] = useState('');
  const joinTokenRef = useRef<JoinToken>(undefined);
  const [tags, setTags] = useState<AWSLabels>([]);
  const [showRestOfSteps, setShowRestOfSteps] = useState(false);

  const [attempt, createJoinTokenAndDiscoveryConfig, setAttempt] = useAsync(
    async () => {
      try {
        const joinTokenService = new JoinTokenService();
        // Don't create another token if token was already created.
        // This can happen if creating discovery config attempt failed
        // and the user retries.
        if (!joinTokenRef.current) {
          joinTokenRef.current = await joinTokenService.fetchJoinTokenV2({
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
              tags: makeTags(tags),
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

        emitEvent(
          { stepStatus: DiscoverEventStatus.Success },
          {
            eventName: DiscoverEvent.CreateDiscoveryConfig,
          }
        );

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
      integrationName: agentMeta.awsIntegration.name,
      accountID: awsAccountId,
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
    <Validation>
      {({ validator }) => (
        <>
          <Header>Setup Discovery Config for Teleport Discovery Service</Header>
          {cfg.isCloud ? (
            <Text>
              The Teleport Discovery Service can connect to Amazon EC2 and
              automatically discover and enroll EC2 instances.{' '}
              <SsmInfoHeaderText />
            </Text>
          ) : (
            <Text>
              Discovery config defines the setup that enables Teleport to
              automatically discover and register instances.{' '}
              <SsmInfoHeaderText />
            </Text>
          )}
          {cfg.isCloud && <SingleEc2InstanceInstallation />}
          <StyledBox mt={4}>
            <header>
              <H3>Step 1</H3>
              <Subtitle3>
                Select the AWS Region that contains the EC2 instances that you
                would like to enroll
              </Subtitle3>
            </header>
            <Box mb={-5}>
              <AwsRegionSelector
                onFetch={(region: Regions) => setSelectedRegion(region)}
                clear={clear}
                disableSelector={!!scriptUrl}
              />
            </Box>
            {selectedRegion && (
              <Box mb={3} mt={5}>
                <P>
                  You can filter for EC2 instances by their tags. If no tags are
                  added, Teleport will enroll all EC2 instances.
                </P>
                <LabelsInput
                  adjective="tag"
                  labels={tags}
                  setLabels={setTags}
                />
              </Box>
            )}
            {!showRestOfSteps && (
              <ButtonSecondary
                onClick={() => setShowRestOfSteps(true)}
                disabled={!selectedRegion}
                data-testid="region-next"
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
                <H3>Step 2</H3>
                <P>
                  Attach AWS managed{' '}
                  <ExternalLink
                    target="_blank"
                    href="https://docs.aws.amazon.com/aws-managed-policy/latest/reference/AmazonSSMManagedInstanceCore.html"
                  >
                    AmazonSSMManagedInstanceCore
                  </ExternalLink>{' '}
                  policy to EC2 instances IAM profile. The policy enables EC2
                  instances to use SSM core functionality.
                </P>
              </StyledBox>
              <StyledBox mt={4}>
                <H3>Step 3</H3>
                <P>
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
                  will list all instances that have SSM agent already running.
                  Ensure ping statuses are <Mark>Online</Mark>.
                </P>
                <Info mt={3} mb={0}>
                  If you do not see your instances listed in the dashboard, it
                  might take up to 30 minutes for your instances to use the IAM
                  credentials you updated in step 2.
                </Info>
              </StyledBox>
              <form>
                <StyledBox mt={4}>
                  <H3>Step 4</H3>
                  <Box>
                    <P mb={3}>
                      Give a name for the{' '}
                      <ExternalLink
                        target="_blank"
                        href="https://docs.aws.amazon.com/systems-manager/latest/userguide/documents.html"
                      >
                        AWS SSM Document
                      </ExternalLink>{' '}
                      that will be created on your behalf. Required to run the
                      installer script on each discovered instances.
                    </P>
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
                    data-testid="script-next"
                  >
                    {scriptUrl ? 'Edit' : 'Next'}
                  </ButtonSecondary>
                </StyledBox>
              </form>
              {scriptUrl && (
                <StyledBox mt={4}>
                  <H3>Step 5</H3>
                  <Flex alignItems="center" gap={1} mb={2}>
                    <P>
                      Run the command below on your{' '}
                      <ExternalLink
                        href="https://console.aws.amazon.com/cloudshell/home"
                        target="_blank"
                      >
                        AWS CloudShell
                      </ExternalLink>{' '}
                      to configure your IAM permissions.
                    </P>
                    <IconTooltip sticky={true} maxWidth={450}>
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
                    </IconTooltip>
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

          {attempt.status === 'error' && (
            <Danger mt={3}>{attempt.statusText}</Danger>
          )}

          <ActionButtons
            onProceed={createJoinTokenAndDiscoveryConfig}
            onPrev={prevStep}
            disableProceed={attempt.status === 'processing' || !scriptUrl}
          />
        </>
      )}
    </Validation>
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
