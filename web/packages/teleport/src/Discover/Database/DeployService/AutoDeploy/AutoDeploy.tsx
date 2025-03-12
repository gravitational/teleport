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

import { useEffect, useState } from 'react';
import styled, { useTheme } from 'styled-components';

import {
  Alert,
  Box,
  ButtonSecondary,
  Link as ExternalLink,
  Flex,
  H3,
  Link,
  Mark,
  Subtitle3,
  Text,
} from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredIamRoleName } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { DbMeta, useDiscover } from 'teleport/Discover/useDiscover';
import type { Database } from 'teleport/services/databases';
import { integrationService, Regions } from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import {
  DiscoverEventStatus,
  DiscoverServiceDeployMethod,
  DiscoverServiceDeployType,
} from 'teleport/services/userEvent';

import {
  ActionButtons,
  AlternateInstructionButton,
  Header,
  HeaderSubtitle,
  TextIcon,
  useShowHint,
} from '../../../Shared';
import awsEcsBblp from '../../aws-ecs-bblp.svg';
import awsEcsDark from '../../aws-ecs-dark.svg';
import awsEcsLight from '../../aws-ecs-light.svg';
import { DeployServiceProp } from '../DeployService';
import { SelectSecurityGroups } from './SelectSecurityGroups';
import { SelectSubnetIds } from './SelectSubnetIds';

export function AutoDeploy({ toggleDeployMethod }: DeployServiceProp) {
  const { emitErrorEvent, nextStep, emitEvent, agentMeta, updateAgentMeta } =
    useDiscover();
  const { attempt, setAttempt } = useAttempt('');

  const [taskRoleArn, setTaskRoleArn] = useState('TeleportDatabaseAccess');
  const [svcDeployedAwsUrl, setSvcDeployedAwsUrl] = useState('');
  const [deployFinished, setDeployFinished] = useState(false);

  // TODO(lisa): look into using validator.Validate() instead
  // of manually validating by hand.
  const [hasNoSubnets, setHasNoSubnets] = useState(false);
  const [hasNoSecurityGroups, setHasNoSecurityGroups] = useState(false);

  const [selectedSubnetIds, setSelectedSubnetIds] = useState<string[]>([]);
  const [selectedSecurityGroups, setSelectedSecurityGroups] = useState<
    string[]
  >([]);

  const dbMeta = agentMeta as DbMeta;

  function manuallyValidateRequiredFields() {
    if (selectedSubnetIds.length === 0) {
      setHasNoSubnets(true);
      return false;
    } else {
      setHasNoSubnets(false);
    }

    if (selectedSecurityGroups.length === 0) {
      setHasNoSecurityGroups(true);
      return false;
    } else {
      setHasNoSecurityGroups(false);
    }

    return true; // valid
  }

  function handleDeploy(validator) {
    setSvcDeployedAwsUrl('');
    setDeployFinished(false);

    if (!validator.validate()) {
      return;
    }

    if (!manuallyValidateRequiredFields()) {
      return;
    }

    const integrationName = dbMeta.awsIntegration.name;
    const { awsAccountId } = splitAwsIamArn(
      agentMeta.awsIntegration.spec.roleArn
    );

    const deployment = {
      region: dbMeta.awsRegion,
      accountId: awsAccountId,
      taskRoleArn,
      deployments: [
        {
          vpcId: dbMeta.awsVpcId,
          subnetIds: selectedSubnetIds,
          securityGroups: selectedSecurityGroups,
        },
      ],
    };

    if (wantAutoDiscover) {
      setAttempt({ status: 'processing' });

      integrationService
        .deployDatabaseServices(integrationName, deployment)
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
      setAttempt({ status: 'processing' });
      integrationService
        .deployDatabaseServices(integrationName, deployment)
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
  const isProcessing = attempt.status === 'processing' && !svcDeployedAwsUrl;
  const isDeploying = attempt.status === 'processing' && !!svcDeployedAwsUrl;
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

            <StyledBox mb={5}>
              <header>
                <H3>Step 2</H3>
              </header>
              <SelectSubnetIds
                selectedSubnetIds={selectedSubnetIds}
                onSelectedSubnetIds={setSelectedSubnetIds}
                dbMeta={dbMeta}
                emitErrorEvent={emitErrorEvent}
                disabled={isProcessing}
              />
            </StyledBox>

            <StyledBox mb={5}>
              <header>
                <H3>Step 3</H3>
              </header>
              <SelectSecurityGroups
                selectedSecurityGroups={selectedSecurityGroups}
                setSelectedSecurityGroups={setSelectedSecurityGroups}
                dbMeta={dbMeta}
                disabled={isProcessing}
                emitErrorEvent={emitErrorEvent}
              />
            </StyledBox>

            <StyledBox mb={5}>
              <header>
                <H3>Step 4</H3>
                <Subtitle3 mb={2}>
                  Deploy the Teleport Database Service
                </Subtitle3>
              </header>
              <ButtonSecondary
                width="230px"
                type="submit"
                onClick={() => handleDeploy(validator)}
                disabled={isProcessing}
                mt={2}
                mb={2}
              >
                {isDeploying
                  ? 'Redeploy Teleport Service'
                  : 'Deploy Teleport Service'}
              </ButtonSecondary>
              {hasError && (
                <Box>
                  <TextIcon mt={3}>
                    <AlertIcon />
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
                region={dbMeta.awsRegion}
              />
            )}

            {wantAutoDiscover && svcDeployedAwsUrl && (
              <AutoDiscoverDeploySuccess
                svcDeployedAwsUrl={svcDeployedAwsUrl}
              />
            )}

            {hasNoSubnets && selectedSubnetIds.length === 0 && (
              <TextIcon mt={3}>
                <AlertIcon />
                At least one subnet selection is required
              </TextIcon>
            )}
            {hasNoSecurityGroups && selectedSecurityGroups.length === 0 && (
              <TextIcon mt={3}>
                <AlertIcon />
                At least one security group selection is required
              </TextIcon>
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
  const theme = useTheme();
  let img = theme.type === 'light' ? awsEcsLight : awsEcsDark;
  if (theme.isCustomTheme && theme.name === 'bblp') {
    img = awsEcsBblp;
  }
  return (
    <>
      <Header>Automatically Deploy a Database Service</Header>
      <HeaderSubtitle>
        Teleport needs a database service to be able to connect to your
        database. Teleport can configure the permissions required to spin up an
        ECS Fargate container (2vCPU, 4GB memory) in your Amazon account with
        the ability to access databases in this region (<Mark>{region}</Mark>).
        You will only need to do this once per geographical region.
      </HeaderSubtitle>
      <Box mb={5} mt={-3}>
        <Box minWidth="500px" maxWidth="998px">
          <img src={img} width="100%" />
        </Box>
        {!wantAutoDiscover && (
          <>
            <br />
            Do you want to deploy a database service manually from one of your
            existing servers?{' '}
            <AlternateInstructionButton
              onClick={toggleDeployMethod}
              disabled={togglerDisabled}
            />
          </>
        )}
      </Box>
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
  const { awsAccountId: accountID } = splitAwsIamArn(
    awsIntegration.spec.roleArn
  );

  function generateAutoConfigScript() {
    if (!validator.validate()) {
      return;
    }

    const newScriptUrl = cfg.getDeployServiceIamConfigureScriptUrl({
      integrationName: awsIntegration.name,
      region: awsRegion,
      // arn's are formatted as `don-care-about-this-part/role-name`.
      // We are splitting by slash and getting the last element.
      awsOidcRoleArn: awsIntegration.spec.roleArn.split('/').pop(),
      taskRoleArn,
      accountID,
    });

    setScriptUrl(newScriptUrl);
  }

  return (
    <StyledBox mb={5}>
      <H3 mb={2}>Step 1</H3>
      <P mb={2}>
        Name an IAM role for the Teleport Database Service and generate a
        configuration command. The generated command will create the role and
        configure permissions for it in your AWS account.
      </P>
      <FieldInput
        mb={4}
        disabled={disabled}
        rule={requiredIamRoleName}
        label="Name an IAM role"
        autoFocus
        value={taskRoleArn}
        placeholder="TeleportDatabaseAccess"
        width="440px"
        mr="3"
        onChange={e => setTaskRoleArn(e.target.value)}
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
  region,
}: {
  resourceName: string;
  deployFinished(dbResult: Database): void;
  abortDeploying(): void;
  svcDeployedAwsUrl: string;
  region: Regions;
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
    const details = (
      <Flex flexDirection="column" gap={3}>
        <Text>
          Visit your AWS{' '}
          <Link target="_blank" href={svcDeployedAwsUrl}>
            dashboard
          </Link>{' '}
          to see progress details.
        </Text>
        <Text>
          There are a few possible reasons for why we haven't been able to
          detect your database service:
        </Text>
        <ul
          css={`
            margin: 0;
            padding-left: ${p => p.theme.space[3]}px;
          `}
        >
          <li>
            The subnets you selected do not route to an internet gateway (igw)
            or a NAT gateway in a public subnet.
          </li>
          <li>
            The security groups you selected do not allow outbound traffic (eg:{' '}
            <Mark>0.0.0.0/0</Mark>) to pull the public Teleport image and to
            reach your Teleport cluster.
          </li>
          <li>
            The security groups attached to your database(s) neither allow
            inbound traffic from the security group you selected nor allow
            inbound traffic from all IPs in the subnets you selected.
          </li>
          <li>
            There may be issues in the region you selected ({region}). Check the{' '}
            <ExternalLink
              target="_blank"
              href="https://health.aws.amazon.com/health/status"
            >
              AWS Health Dashboard
            </ExternalLink>{' '}
            for any problems.
          </li>
          <li>
            The network may be slow. Try waiting for a few more minutes or{' '}
            <AlternateInstructionButton onClick={abortDeploying}>
              try manually deploying your own database service.
            </AlternateInstructionButton>
          </li>
        </ul>
        <Text>
          Refer to the{' '}
          <Link
            target="_blank"
            href="https://goteleport.com/docs/admin-guides/management/guides/awsoidc-integration-rds/#troubleshooting"
          >
            troubleshooting documentation
          </Link>{' '}
          for more details.
        </Text>
      </Flex>
    );
    return (
      <Alert kind="warning" alignItems="flex-start" details={details}>
        We&apos;re still in the process of creating your database service
      </Alert>
    );
  }

  if (result) {
    return (
      <Alert kind="success" dismissible={false}>
        Successfully created and detected your new database service.
      </Alert>
    );
  }

  const details = (
    <Text>
      It will take at least a minute for the Database Service to be created and
      joined to your cluster. <br />
      We will update this status once detected, meanwhile visit your AWS{' '}
      <Link target="_blank" href={svcDeployedAwsUrl}>
        dashboard
      </Link>{' '}
      to see progress details.
    </Text>
  );
  return (
    <Alert
      kind="neutral"
      alignItems="flex-start"
      icon={Icons.Restore}
      dismissible={false}
      details={details}
    >
      Teleport is currently deploying a database service
    </Alert>
  );
};

export function AutoDiscoverDeploySuccess({
  svcDeployedAwsUrl,
}: {
  svcDeployedAwsUrl: string;
}) {
  const details = (
    <>
      Discovery will complete in a minute. You can visit your AWS{' '}
      <Link target="_blank" href={svcDeployedAwsUrl}>
        dashboard
      </Link>{' '}
      to see progress details.
    </>
  );
  return (
    <Alert kind="success" dismissible={false} details={details}>
      The required database services have been deployed successfully.
    </Alert>
  );
}

const StyledBox = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;

const AlertIcon = () => (
  <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
);
