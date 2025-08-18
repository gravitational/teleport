/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useMutation } from '@tanstack/react-query';
import { ChangeEvent, ReactNode, useMemo, useState } from 'react';
import { useHistory } from 'react-router';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  CardTile,
  Flex,
  H2,
  Link,
  Subtitle2,
} from 'design';
import { CollapsibleInfoSection } from 'design/CollapsibleInfoSection';
import { Info, NewTab } from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { InfoGuideButton } from 'shared/components/SlidingSidePanel/InfoGuide';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { FeatureBox } from 'teleport/components/Layout';
import cfg from 'teleport/config';
import { Guide } from 'teleport/Integrations/Enroll/AwsConsole/IamIntegration/Guide';
import { validTrustAnchorInput } from 'teleport/Integrations/Enroll/AwsConsole/rules';
import {
  cloudShell,
  rolesAnywhere,
} from 'teleport/Integrations/Enroll/awsLinks';
import {
  AwsRolesAnywherePingResponse,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import useTeleport from 'teleport/useTeleport';

enum Phase {
  One, // enable step one
  Two, // enable step two & three
  Three, // step three is verified, enable proceed button
}

export function IamIntegration() {
  const ctx = useTeleport();
  const integrationsAccess = ctx.storeUser.getIntegrationsAccess();
  const canEnroll = integrationsAccess.create;
  const clusterId = ctx.storeUser.getClusterId();
  const resourceRoute = cfg.getUnifiedResourcesRoute(clusterId);

  const history = useHistory();
  const [phase, setPhase] = useState(Phase.One);
  const [alert, setAlert] = useState<ReactNode>();
  const [scriptUrl, setScriptUrl] = useState<string>();
  const [sync, setSync] = useState<string>();
  const [output, setOutput] = useState<{
    trustAnchorArn: string;
    syncRoleArn: string;
    syncProfileArn: string;
  }>();

  const [formState, setFormState] = useState({
    // integrationName: the name of the AWS IAM Roles Anywhere Integration
    // required by user, no default
    integrationName: '',
    // trustAnchor: the name of the Trust Anchor to be created
    trustAnchorName: 'teleport-aws-roles-anywhere-trust-anchor',
    // syncRole: the name of the IAM Role to be created
    syncProfileName: 'teleport-aws-roles-anywhere-profile',
    // syncProfile: the name of the IAM Roles Anywhere Profile to be created
    syncRoleName: 'teleport-aws-roles-anywhere-role',
  });

  function generateCommand(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    setScriptUrl(
      cfg.getAwsRolesAnywhereGenerateUrl(
        formState.integrationName,
        formState.trustAnchorName,
        formState.syncRoleName,
        formState.syncProfileName
      )
    );
    setPhase(Phase.Two);
  }

  const manageAction = {
    content: (
      <>
        Manage AWS Roles Anywhere Profiles{' '}
        <NewTab size={18} ml={2} style={{ lineHeight: '22px' }} />
      </>
    ),
    href: rolesAnywhere,
  };

  const testCfg = useMutation({
    mutationFn: () => {
      const inputs = parseOutput(sync);
      // set output for final redirect
      setOutput({
        trustAnchorArn: inputs.trustAnchorArn,
        syncProfileArn: inputs.syncProfileArn,
        syncRoleArn: inputs.syncRoleArn,
      });
      return integrationService.awsRolesAnywherePing({
        integrationName: formState.integrationName,
        trustAnchorArn: inputs.trustAnchorArn,
        syncRoleArn: inputs.syncRoleArn,
        syncProfileArn: inputs.syncProfileArn,
      });
    },
    onError: error => {
      setAlert(
        <Alert kind="danger" details={error.message}>
          Error: {error.name}
        </Alert>
      );
    },
    onSuccess: (response: AwsRolesAnywherePingResponse) => {
      if (response.profileCount > 0) {
        setAlert(
          <Alert kind="success" secondaryAction={manageAction}>
            Success! AWS IAM Roles Anywhere Profiles found
          </Alert>
        );
      } else {
        setAlert(
          <Alert
            kind="warning"
            secondaryAction={manageAction}
            details={
              'Create your first Profile to start accessing AWS from Teleport.'
            }
          >
            Teleport didn&#39;t find any Profiles
          </Alert>
        );
      }

      setPhase(Phase.Three);
    },
  });

  const validInput = useMemo(() => {
    return validTrustAnchorInput(sync)().valid;
  }, [sync]);

  if (!canEnroll) {
    return (
      <FeatureBox>
        <Alert kind="info" mt={4}>
          You do not have permission to enroll integrations. Missing role
          permissions: <code>integrations.create</code>
        </Alert>
      </FeatureBox>
    );
  }

  return (
    <>
      <Flex mt={3} justifyContent="space-between" alignItems="center">
        <H2>AWS CLI / Console Access</H2>
        <InfoGuideButton
          config={{
            guide: <Guide resourcesRoute={resourceRoute} />,
          }}
        />
      </Flex>
      <Box>
        Compatible with any CLI and AWS SDK-based tooling (such as Terraform and
        AWS CLI). Teleport uses <strong>AWS IAM Roles Anywhere</strong> to
        manage access and allows you to configure the right permissions for your
        users.
        <br />
        Follow the below steps to create a Roles Anywhere Trust Anchor and
        configure the required IAM Roles for synchronizing Profiles as Teleport
        resources.
      </Box>
      <Alert
        kind="neutral"
        icon={Info}
        mt={5}
        secondaryAction={manageAction}
        details={
          'Create AWS Profiles and assign Roles to them in your AWS account. Teleport will allow you to import these Profiles as Resources.'
        }
      >
        Prerequisites
      </Alert>
      <Validation>
        {({ validator }) => (
          <Flex flexDirection="column" gap={3}>
            <CardTile>
              <Flex flexDirection="column">
                <H2>Step 1: Name your Teleport Integration</H2>
                <Subtitle2>Give this integration a name.</Subtitle2>
              </Flex>
              <Flex flexDirection="column" gap={1} maxWidth={500}>
                <FieldInput
                  readonly={phase !== Phase.One}
                  label="Integration Name"
                  placeholder="teleport-aws-prod"
                  rule={requiredField('Name is required')}
                  value={formState.integrationName}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => {
                    return setFormState(prev => ({
                      ...prev,
                      integrationName: e.target.value,
                    }));
                  }}
                />
              </Flex>
              <CollapsibleInfoSection
                openLabel="Optionally edit Trust Anchor, IAM Role and Profile Names"
                closeLabel="Edit Trust Anchor, IAM Role and Profile Names"
              >
                <Flex flexDirection="column" gap={1} maxWidth={500}>
                  <FieldInput
                    readonly={phase !== Phase.One}
                    label="Trust Anchor Name"
                    placeholder=""
                    value={formState.trustAnchorName}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => {
                      return setFormState(prev => ({
                        ...prev,
                        trustAnchorName: e.target.value,
                      }));
                    }}
                  />
                  <FieldInput
                    readonly={phase !== Phase.One}
                    label="AWS IAM Role Name *"
                    placeholder=""
                    value={formState.syncRoleName}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => {
                      return setFormState(prev => ({
                        ...prev,
                        syncRoleName: e.target.value,
                      }));
                    }}
                  />
                  <FieldInput
                    readonly={phase !== Phase.One}
                    label="Roles Anywhere Profile Name *"
                    placeholder=""
                    value={formState.syncProfileName}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => {
                      return setFormState(prev => ({
                        ...prev,
                        syncProfileName: e.target.value,
                      }));
                    }}
                  />
                </Flex>
              </CollapsibleInfoSection>
              <ButtonPrimary
                width="200px"
                disabled={phase !== Phase.One}
                onClick={() => generateCommand(validator)}
              >
                Generate Command
              </ButtonPrimary>
            </CardTile>
            {phase !== Phase.One && (
              <>
                <CardTile>
                  <Flex flexDirection="column">
                    <H2>Step 2: Create Roles Anywhere Trust Anchor</H2>
                    <Subtitle2>
                      Open{' '}
                      <Link href={cloudShell} target="_blank">
                        AWS CloudShell
                      </Link>{' '}
                      and copy and paste the command below. Upon executing in
                      the AWS Shell the command will download and execute
                      Teleport binary that configures Teleport as a IAM Roles
                      Anywhere trusted entity. After running the script, copy
                      the output and paste it in the field below.
                    </Subtitle2>
                  </Flex>
                  <Box mb={2} mt={3}>
                    <TextSelectCopyMulti
                      lines={[
                        {
                          text: `bash -c "$(curl '${scriptUrl}')"`,
                        },
                      ]}
                    />
                  </Box>
                </CardTile>
                <CardTile>
                  <Flex flexDirection="column">
                    <H2>
                      Step 3: Create and Sync the Integration Profile and Role
                    </H2>
                    <Subtitle2>
                      Once the script above completes, copy and paste its output
                      below.
                    </Subtitle2>
                    <Subtitle2>
                      You can run the script again if you’ve already closed the
                      window.
                    </Subtitle2>
                  </Flex>
                  <FieldTextArea
                    width="750px"
                    value={sync}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => {
                      setSync(e.target.value);
                    }}
                    label="Trust Anchor, Profile and Role ARNs"
                    placeholder={`:trust-anchor/\n:profile/\n:role/\n`}
                    rule={validTrustAnchorInput}
                  />
                  <ButtonPrimary
                    width="200px"
                    onClick={() => testCfg.mutate()}
                    disabled={!sync || phase !== Phase.Two || !validInput}
                  >
                    Test Configuration
                  </ButtonPrimary>
                  {alert && alert}
                </CardTile>
              </>
            )}
            <Flex gap={3}>
              <ButtonPrimary
                width="200px"
                disabled={phase !== Phase.Three}
                onClick={() =>
                  history.push(
                    cfg.getIntegrationEnrollRoute(
                      IntegrationKind.AWSRa,
                      'access'
                    ),
                    {
                      integrationName: formState.integrationName,
                      trustAnchorArn: output.trustAnchorArn,
                      syncRoleArn: output.syncRoleArn,
                      syncProfileArn: output.syncProfileArn,
                    }
                  )
                }
              >
                Next: Configure Access
              </ButtonPrimary>
              <ButtonSecondary onClick={history.goBack} width="100px">
                Back
              </ButtonSecondary>
            </Flex>
          </Flex>
        )}
      </Validation>
    </>
  );
}

// The backend will use AWS validation to ensure the params are valid, we just want to get close at this point
// Will remove dividing line if present, then ensures there are three inputs each starting with `arn:aws:`
export function parseOutput(value: string): {
  trustAnchorArn: string;
  syncProfileArn: string;
  syncRoleArn: string;
} {
  if (!value) {
    return;
  }

  const lines = value.split('\n');
  const trustAnchor = lines
    .find(l => l.includes(':trust-anchor/'))
    .replace(/"/g, '')
    .trim();
  const profile = lines
    .find(l => l.includes(':profile/'))
    .replace(/"/g, '')
    .trim();
  const role = lines
    .find(l => l.includes(':role/'))
    .replace(/"/g, '')
    .trim();

  return {
    trustAnchorArn: trustAnchor,
    syncProfileArn: profile,
    syncRoleArn: role,
  };
}
