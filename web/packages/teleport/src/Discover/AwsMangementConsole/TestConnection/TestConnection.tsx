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

import React, { useState } from 'react';
import { Text, Box, Link, Mark } from 'design';
import Select from 'shared/components/Select';
import { OutlineInfo } from 'design/Alert/Alert';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';

import cfg from 'teleport/config';
import { generateTshLoginCommand, openNewTab } from 'teleport/lib/util';
import {
  Header,
  ActionButtons,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import useTeleport from 'teleport/useTeleport';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';
import { AWS_TAG_INFO_LINK } from 'teleport/Discover/Shared/const';

import { AppMeta, useDiscover } from '../../useDiscover';

import type { Option } from 'shared/components/Select';

export function TestConnection() {
  const ctx = useTeleport();
  const { username, authType } = ctx.storeUser.state;
  const clusterId = ctx.storeUser.getClusterId();

  const { nextStep, prevStep, agentMeta } = useDiscover();
  const { app } = agentMeta as AppMeta;

  const arnOpts = app.awsRoles.map(({ arn }) => ({ value: arn, label: arn }));
  const [selectedOpt, setSelectedOpt] = useState<Option>();

  function launchUrl(arn: string) {
    const { fqdn, clusterId, publicAddr } = app;
    const appUrl = cfg.getAppLauncherRoute({
      fqdn,
      clusterId,
      publicAddr,
      arn,
    });

    openNewTab(appUrl);
  }

  let arnResourceName = '<IAM-Role-Name>';
  const splittedArn = splitAwsIamArn(selectedOpt?.value);
  if (splittedArn.arnResourceName) {
    arnResourceName = splittedArn.arnResourceName;
  }
  const tshCmd = `tsh apps login --aws-role ${arnResourceName} ${app.name}`;

  return (
    <Box>
      <Header>Test Connection</Header>
      <HeaderSubtitle>
        Optionally, verify that you can successfully connect to the application
        you just added.
      </HeaderSubtitle>
      <StyledBox mb={5}>
        <Text bold mb={3}>
          Access the AWS Management Console
        </Text>
        <Text typography="subtitle1" mb={2}>
          Select the AWS role ARN to test.
        </Text>
        <Text typography="subtitle1" mb={2}>
          AWS Management Console will launch in another tab. You should see your
          Teleport user name as a federated login with the selected role in the
          top-right corner of the AWS Console.
        </Text>
        <Box width="500px">
          <Select
            value={selectedOpt}
            options={arnOpts}
            onChange={(o: Option) => {
              setSelectedOpt(o);
              launchUrl(o.value);
            }}
          />
        </Box>
      </StyledBox>
      <StyledBox mb={5}>
        <Text bold mb={3}>
          Access the AWS CLI
        </Text>
        <Box mb={2}>
          Log into your Teleport cluster
          <TextSelectCopy
            mt="1"
            text={generateTshLoginCommand({
              authType,
              username,
              clusterId,
            })}
          />
        </Box>
        <Box mb={2}>
          Connect to your application
          <TextSelectCopy mt="1" text={tshCmd} />
        </Box>
      </StyledBox>
      <OutlineInfo mb={3} linkColor="buttons.link.default" width="800px">
        <Text>
          If the connection can't be established, ensure the IAM role you are
          trying to assume is{' '}
          <Link target="_blank" href={AWS_TAG_INFO_LINK}>
            tagged
          </Link>{' '}
          with key <Mark>teleport.dev/integration</Mark> and value{' '}
          <Mark>true</Mark>.
        </Text>
      </OutlineInfo>

      <ActionButtons onProceed={nextStep} lastStep={true} onPrev={prevStep} />
    </Box>
  );
}
