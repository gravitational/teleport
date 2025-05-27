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

import { ChangeEvent, useState } from 'react';
import { useHistory, useParams } from 'react-router';
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
} from 'web/packages/design/src';
import { Info, NewTab } from 'web/packages/design/src/Icon';

import FieldInput from 'shared/components/FieldInput';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { IntegrationKind } from 'teleport/services/integrations';

enum Phase {
  One, // enable step one
  Two, // enable step two & three
  Three, // step three is verified, enable proceed
}

export function IamIntegration() {
  const { name } = useParams<{ name: string }>();
  const history = useHistory();
  const [phase, setPhase] = useState(Phase.One);
  const [integrationName, setIntegrationName] = useState<string>();
  const [scriptUrl, setScriptUrl] = useState<string>();
  const [sync, setSync] = useState<string>();

  function generateCommand(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    // todo mberg generate command, check errors
    setPhase(Phase.Two);
    setScriptUrl('example.com');
  }

  function testCfg() {
    // todo mberg check for errors or show success
    // todo marco; we show manage profiles here; what is the implication for the next page if something changes?
    setPhase(Phase.Three);
  }

  // todo mberg in-guide
  return (
    <Box pt={3}>
      <H2>AWS CLI / Console Access</H2>
      <Box mb={4}>
        Compatible with any CLI and AWS SDK-based tootling (includes Terraform,
        AWS CLI). Teleport uses <b>AWS IAM Roles Anywhere</b> to manage access
        and allows you to configure the right permissions for your users.
        <br />
        Follow the below steps to create a Roles Anywhere Trust Anchor and
        configure the required IAM Roles for synchronizing Profiles as Teleport
        resources.
      </Box>
      <Alert
        kind="neutral"
        icon={Info}
        mt={5}
        secondaryAction={{
          content: (
            <>
              Manage AWS Roles Anywhere Profiles
              <NewTab size={18} ml={2} />
            </>
          ),
          onClick: () => console.log('todo mberg'),
        }}
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
                  label="Integration Name*"
                  placeholder="teleport-aws-prod"
                  rule={requiredField('Name is required')}
                  value={integrationName}
                  onChange={(e: ChangeEvent<HTMLInputElement>) =>
                    setIntegrationName(e.target.value)
                  }
                />
              </Flex>
              {/* todo (michellescripts) optionally edit aws fields */}
              {/* todo marco - how are these "editable" at this point & what's the implication ? */}
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
                      <Link
                        href="https://console.aws.amazon.com/cloudshell/home"
                        target="_blank"
                      >
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
                    onChange={(e: ChangeEvent<HTMLInputElement>) =>
                      setSync(e.target.value)
                    }
                    label="Trust Anchor, Profile and Role ARNs"
                    placeholder={`:trust-anchor/
:profile/
:role/`}
                  />
                  <ButtonPrimary
                    width="200px"
                    onClick={testCfg}
                    disabled={!sync || phase !== Phase.Two}
                  >
                    Test Configuration
                  </ButtonPrimary>
                </CardTile>
              </>
            )}
            <Flex gap={3}>
              <ButtonPrimary
                width="200px"
                disabled={phase !== Phase.Three}
                onClick={() =>
                  history.push(
                    cfg.getIntegrationEnrollChildRoute(
                      IntegrationKind.AwsOidc,
                      name,
                      IntegrationKind.AwsConsole,
                      'access'
                    )
                  )
                }
              >
                Next: Configure Access
              </ButtonPrimary>
              <ButtonSecondary
                onClick={() =>
                  history.push(
                    cfg.getIntegrationStatusRoute(IntegrationKind.AwsOidc, name)
                  )
                }
                width="100px"
              >
                Back
              </ButtonSecondary>
            </Flex>
          </Flex>
        )}
      </Validation>
    </Box>
  );
}
