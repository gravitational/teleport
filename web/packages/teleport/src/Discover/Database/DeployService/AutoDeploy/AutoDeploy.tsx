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

import React, { useState, useEffect } from 'react';
import styled from 'styled-components';
import { Box, ButtonSecondary, Link, Text, Mark } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';
import { requiredIamRoleName } from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import {
  HintBox,
  SuccessBox,
  WaitingInfo,
} from 'teleport/Discover/Shared/HintBox';
import { integrationService } from 'teleport/services/integrations';
import { useDiscover, DbMeta } from 'teleport/Discover/useDiscover';
import {
  DiscoverEventStatus,
  DiscoverServiceDeployMethod,
  DiscoverServiceDeployType,
} from 'teleport/services/userEvent';
import cfg from 'teleport/config';

import {
  ActionButtons,
  HeaderSubtitle,
  TextIcon,
  useShowHint,
  Header,
  DiscoverLabel,
  AlternateInstructionButton,
} from '../../../Shared';

import { DeployServiceProp } from '../DeployService';
import { hasMatchingLabels, Labels } from '../../common';

import { SelectSecurityGroups } from './SelectSecurityGroups';

import type { Database } from 'teleport/services/databases';

export function AutoDeploy({ toggleDeployMethod }: DeployServiceProp) {
  const { emitErrorEvent, nextStep, emitEvent, agentMeta, updateAgentMeta } =
    useDiscover();
  const { attempt, setAttempt } = useAttempt('');
  const [showLabelMatchErr, setShowLabelMatchErr] = useState(true);

  const [taskRoleArn, setTaskRoleArn] = useState('TeleportDatabaseAccess');
  const [svcDeployedAwsUrl, setSvcDeployedAwsUrl] = useState('');
  const [deployFinished, setDeployFinished] = useState(false);

  const [selectedSecurityGroups, setSelectedSecurityGroups] = useState<
    string[]
  >([]);

  const hasDbLabels = agentMeta?.agentMatcherLabels?.length;
  const dbLabels = hasDbLabels ? agentMeta.agentMatcherLabels : [];
  const [labels, setLabels] = useState<DiscoverLabel[]>([
    { name: '*', value: '*', isFixed: dbLabels.length === 0 },
  ]);
  const dbMeta = agentMeta as DbMeta;

  useEffect(() => {
    // Turn off error once user changes labels.
    if (showLabelMatchErr) {
      setShowLabelMatchErr(false);
    }
  }, [labels]);

  function handleDeploy(validator) {
    if (!validator.validate()) {
      return;
    }

    const integrationName = dbMeta.awsIntegration.name;

    if (wantAutoDiscover) {
      setAttempt({ status: 'processing' });

      const requiredVpcsAndSubnets =
        dbMeta.autoDiscovery.requiredVpcsAndSubnets;
      const vpcIds = Object.keys(requiredVpcsAndSubnets);

      integrationService
        .deployDatabaseServices(integrationName, {
          region: dbMeta.awsRegion,
          taskRoleArn,
          deployments: vpcIds.map(vpcId => ({
            vpcId,
            subnetIds: requiredVpcsAndSubnets[vpcId],
          })),
        })
        .then(url => {
          setAttempt({ status: 'success' });
          setSvcDeployedAwsUrl(url);
          setDeployFinished(true);
          updateAgentMeta({ ...agentMeta, serviceDeployedMethod: 'auto' });
        })
        .catch((err: Error) => {
          setAttempt({ status: 'failed', statusText: err.message });
          emitErrorEvent(`auto discover deploy request failed: ${err.message}`);
        });
    } else {
      if (!hasMatchingLabels(dbLabels, labels)) {
        setShowLabelMatchErr(true);
        return;
      }

      setShowLabelMatchErr(false);
      setAttempt({ status: 'processing' });
      integrationService
        .deployAwsOidcService(integrationName, {
          deploymentMode: 'database-service',
          region: dbMeta.awsRegion,
          subnetIds: dbMeta.selectedAwsRdsDb?.subnets,
          taskRoleArn,
          databaseAgentMatcherLabels: labels,
          securityGroups: selectedSecurityGroups,
        })
        // The user is still technically in the "processing"
        // state, because after this call succeeds, we will
        // start pinging for the newly registered db
        // to get picked up by this service we deployed.
        // So setting the attempt here to "success"
        // is not necessary.
        .then(url => {
          setSvcDeployedAwsUrl(url);
        })
        .catch((err: Error) => {
          setAttempt({ status: 'failed', statusText: err.message });
          emitErrorEvent(`deploy request failed: ${err.message}`);
        });
    }
  }

  function handleOnProceed() {
    // Auto discover skips the IAM policy view since
    // we ask them to configure it as first step.
    nextStep(2);
    emitEvent(
      { stepStatus: DiscoverEventStatus.Success },
      {
        serviceDeploy: {
          method: DiscoverServiceDeployMethod.Auto,
          type: DiscoverServiceDeployType.AmazonEcs,
        },
      }
    );
  }

  function handleDeployFinished(db: Database) {
    setDeployFinished(true);
    updateAgentMeta({ ...agentMeta, db, serviceDeployedMethod: 'auto' });
  }

  function abortDeploying() {
    if (attempt.status === 'processing') {
      emitErrorEvent(
        `aborted in middle of auto deploying (>= 5 minutes of waiting)`
      );
    }
    setSvcDeployedAwsUrl(null);
    setAttempt({ status: '' });
    toggleDeployMethod();
  }

  const wantAutoDiscover = !!dbMeta.autoDiscovery;
  const isProcessing = attempt.status === 'processing' && !!svcDeployedAwsUrl;
  const isDeploying = isProcessing && !!svcDeployedAwsUrl;
  const hasError = attempt.status === 'failed';

  return (
    <Box>
      <Validation>
        {({ validator }) => (
          <>
            <Heading
              toggleDeployMethod={abortDeploying}
              togglerDisabled={isProcessing}
              region={dbMeta.awsRegion}
              wantAutoDiscover={wantAutoDiscover}
            />

            {/* step one */}
            <CreateAccessRole
              taskRoleArn={taskRoleArn}
              setTaskRoleArn={setTaskRoleArn}
              disabled={isProcessing}
              dbMeta={dbMeta}
              validator={validator}
            />

            {/* step two & step three
             * for auto discover, these steps are disabled atm since
             * user's can't supply custom label matchers and selecting
             * security groups is out of scope.
             */}
            {!wantAutoDiscover && (
              <>
                <StyledBox mb={5}>
                  <Text bold>Step 2 (Optional)</Text>
                  <Labels
                    labels={labels}
                    setLabels={setLabels}
                    disableBtns={attempt.status === 'processing'}
                    showLabelMatchErr={showLabelMatchErr}
                    dbLabels={dbLabels}
                    autoFocus={false}
                    region={dbMeta.selectedAwsRdsDb?.region}
                  />
                </StyledBox>
                {/* step three */}
                <StyledBox mb={5}>
                  <Text bold>Step 3 (Optional)</Text>
                  <SelectSecurityGroups
                    selectedSecurityGroups={selectedSecurityGroups}
                    setSelectedSecurityGroups={setSelectedSecurityGroups}
                    dbMeta={dbMeta}
                    emitErrorEvent={emitErrorEvent}
                  />
                </StyledBox>
              </>
            )}

            <StyledBox mb={5}>
              <Text bold>Step {wantAutoDiscover ? 2 : 4}</Text>
              <Text mb={2}>Deploy the Teleport Database Service.</Text>
              <ButtonSecondary
                width="215px"
                type="submit"
                onClick={() => handleDeploy(validator)}
                disabled={attempt.status === 'processing'}
                mt={2}
                mb={2}
              >
                Deploy Teleport Service
              </ButtonSecondary>
              {hasError && (
                <Box>
                  <TextIcon mt={3}>
                    <Icons.Warning
                      size="medium"
                      ml={1}
                      mr={2}
                      color="error.main"
                    />
                    Encountered Error: {attempt.statusText}
                  </TextIcon>
                  <Text mt={2}>
                    <b>Note:</b> If this is your first attempt, it might be that
                    AWS has not finished propagating changes from{' '}
                    <Mark>Step 1</Mark>. Try waiting a minute before attempting
                    again.
                  </Text>
                </Box>
              )}
            </StyledBox>

            {!wantAutoDiscover && isDeploying && (
              <DeployHints
                deployFinished={handleDeployFinished}
                resourceName={agentMeta.resourceName}
                abortDeploying={abortDeploying}
                svcDeployedAwsUrl={svcDeployedAwsUrl}
              />
            )}

            {wantAutoDiscover && svcDeployedAwsUrl && (
              <AutoDiscoverDeploySuccess
                svcDeployedAwsUrl={svcDeployedAwsUrl}
              />
            )}

            <ActionButtons
              onProceed={handleOnProceed}
              disableProceed={!deployFinished}
            />
          </>
        )}
      </Validation>
    </Box>
  );
}

const Heading = ({
  toggleDeployMethod,
  togglerDisabled,
  region,
  wantAutoDiscover,
}: {
  toggleDeployMethod(): void;
  togglerDisabled: boolean;
  region: string;
  wantAutoDiscover: boolean;
}) => {
  return (
    <>
      <Header>Automatically Deploy a Database Service</Header>
      <HeaderSubtitle>
        Teleport needs a database service to be able to connect to your
        database. Teleport can configure the permissions required to spin up an
        ECS Fargate container (2vCPU, 4GB memory) in your Amazon account with
        the ability to access databases in this region (<Mark>{region}</Mark>).
        You will only need to do this once per geographical region.
        {!wantAutoDiscover && (
          <>
            <br />
            <br />
            Want to deploy a database service manually from one of your existing
            servers?{' '}
            <AlternateInstructionButton
              onClick={toggleDeployMethod}
              disabled={togglerDisabled}
            />
          </>
        )}
      </HeaderSubtitle>
    </>
  );
};

const CreateAccessRole = ({
  taskRoleArn,
  setTaskRoleArn,
  disabled,
  dbMeta,
  validator,
}: {
  taskRoleArn: string;
  setTaskRoleArn(r: string): void;
  disabled: boolean;
  dbMeta: DbMeta;
  validator: Validator;
}) => {
  const [scriptUrl, setScriptUrl] = useState('');
  const { awsIntegration, awsRegion } = dbMeta;

  function generateAutoConfigScript() {
    if (!validator.validate()) {
      return;
    }

    const newScriptUrl = cfg.getDeployServiceIamConfigureScriptUrl({
      integrationName: awsIntegration.name,
      region: awsRegion,
      // arn's are formatted as `don-care-about-this-part/role-arn`.
      // We are splitting by slash and getting the last element.
      awsOidcRoleArn: awsIntegration.spec.roleArn.split('/').pop(),
      taskRoleArn,
    });

    setScriptUrl(newScriptUrl);
  }

  return (
    <StyledBox mb={5}>
      <Text bold>Step 1</Text>
      <Text mb={2}>
        Name a Task Role ARN for this Database Service and generate a configure
        command. This command will configure the required permissions in your
        AWS account.
      </Text>
      <FieldInput
        mb={4}
        disabled={disabled}
        rule={requiredIamRoleName}
        label="Name a Task Role ARN"
        autoFocus
        value={taskRoleArn}
        placeholder="TeleportDatabaseAccess"
        width="440px"
        mr="3"
        onChange={e => setTaskRoleArn(e.target.value)}
        toolTipContent={`Amazon Resource Names (ARNs) uniquely identify AWS \
        resources. In this case you will naming an IAM role that this \
        deployed service will be using`}
      />
      <ButtonSecondary mb={3} onClick={generateAutoConfigScript}>
        {scriptUrl ? 'Regenerate Command' : 'Generate Command'}
      </ButtonSecondary>
      {scriptUrl && (
        <>
          <Text mb={2}>
            Open{' '}
            <Link
              href="https://console.aws.amazon.com/cloudshell/home"
              target="_blank"
            >
              AWS CloudShell
            </Link>{' '}
            and copy/paste the following command:
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
        </>
      )}
    </StyledBox>
  );
};

const DeployHints = ({
  resourceName,
  deployFinished,
  abortDeploying,
  svcDeployedAwsUrl,
}: {
  resourceName: string;
  deployFinished(dbResult: Database): void;
  abortDeploying(): void;
  svcDeployedAwsUrl: string;
}) => {
  // Starts resource querying interval.
  const { result, active } = usePingTeleport<Database>(resourceName);

  const showHint = useShowHint(active);

  useEffect(() => {
    if (result) {
      deployFinished(result);
    }
  }, [result]);

  if (showHint && !result) {
    return (
      <HintBox header="We're still in the process of creating your Database Service">
        <Text mb={3}>
          The network may be slow. Try continuing to wait for a few more minutes
          or{' '}
          <AlternateInstructionButton onClick={abortDeploying}>
            try manually deploying your own service.
          </AlternateInstructionButton>{' '}
          You can visit your AWS{' '}
          <Link target="_blank" href={svcDeployedAwsUrl}>
            dashboard
          </Link>{' '}
          to see progress details.
        </Text>
      </HintBox>
    );
  }

  if (result) {
    return (
      <SuccessBox>
        Successfully created and detected your new Database Service.
      </SuccessBox>
    );
  }

  return (
    <WaitingInfo>
      <TextIcon
        css={`
          white-space: pre;
          margin-right: 4px;
          padding-right: 4px;
        `}
      >
        <Icons.Restore size="medium" mr={1} />
      </TextIcon>
      <Text>
        Teleport is currently deploying a Database Service. It will take at
        least a minute for the Database Service to be created and joined to your
        cluster. <br />
        We will update this status once detected, meanwhile visit your AWS{' '}
        <Link target="_blank" href={svcDeployedAwsUrl}>
          dashboard
        </Link>{' '}
        to see progress details.
      </Text>
    </WaitingInfo>
  );
};

export function AutoDiscoverDeploySuccess({
  svcDeployedAwsUrl,
}: {
  svcDeployedAwsUrl: string;
}) {
  return (
    <SuccessBox>
      The required database services have been deployed successfully. Discovery
      will complete in a minute. You can visit your AWS{' '}
      <Link target="_blank" href={svcDeployedAwsUrl}>
        dashboard
      </Link>{' '}
      to see progress details.
    </SuccessBox>
  );
}

const StyledBox = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
