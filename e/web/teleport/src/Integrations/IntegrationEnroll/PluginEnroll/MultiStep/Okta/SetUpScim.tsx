import styled from 'styled-components';

import { Box, ButtonPrimary, Flex, Link, Mark, Text } from 'design';
import { OutlineInfo } from 'design/Alert/Alert';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import { PluginOktaSpec } from 'teleport/services/integrations';

import { Header } from '../Shared';
import { usePlugin } from '../usePlugin';
import { FormDataField } from './types';

export function SetUpScim() {
  const { nextStep, formData, installedPlugin } = usePlugin<PluginOktaSpec>();

  function onFinish() {
    nextStep();
  }

  // TODO(lisa): this is hard coded for now and is equal to backend
  // hard coded value. Next iteration we will allow user to name
  // the SSO connector.
  const ssoConnectorName = 'okta';

  // Construct okta app "admin" URL.
  // Admin URL's have `-admin` before the domain's `okta.com`.
  let orgUrl = formData
    .get(FormDataField.OrgUrl)
    .toString()
    .replace(/.okta.com[/]?$/, '');
  const appUrl = `${orgUrl}-admin.okta.com/admin/app/${installedPlugin.spec.oktaAppName}/instance/${installedPlugin.spec.oktaAppId}/#tab-general`;

  return (
    <Box width="800px">
      <Box mb={3}>
        <Header header="Set Up SCIM" />
        <Text>
          Okta SCIM (System for Cross-domain Identity Management) integration
          configures Okta to push user and permission changes in Okta to
          Teleport in real time. To get it working, follow the directions below:
        </Text>
      </Box>
      <Flex mb={1} mt={4} flexDirection="column" gap={4} width="100%">
        <StyledBox>
          <Text bold>Step 1: Enable SCIM</Text>
          <Text>
            In the Okta Admin Console, go to <Mark>Applications</Mark> and click
            on Teleport created application, or use this{' '}
            <Link href={appUrl} target="_blank">
              link
            </Link>
          </Text>
          <Text mt={2}>
            On <Mark>General</Mark> tab, click <Mark>Edit</Mark> under{' '}
            <Mark>App Settings</Mark> and select the <Mark>SCIM</Mark> option in{' '}
            <Mark>Provisioning</Mark> and click <Mark>Save</Mark>
          </Text>
        </StyledBox>
        <StyledBox>
          <Text bold>Step 2: Setup SCIM Connection</Text>
          <Text>
            Click on the <Mark>Provisioning</Mark> tab and click{' '}
            <Mark>Edit</Mark>
          </Text>
          <Box mt={2}>
            <Text mb={2}>
              Copy and paste this <Mark>SCIM connector base URL</Mark>:
            </Text>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text: `${cfg.baseUrl}/v1/webapi/scim/${ssoConnectorName}`,
                },
              ]}
            />
          </Box>
          <Box mt={2}>
            <Text mb={2}>
              Copy and paste this <Mark>Unique identifier field for users</Mark>
              :
            </Text>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text: `userName`,
                },
              ]}
            />
          </Box>
          <Text mt={3}>
            Under <Mark>Supported provisioning actions</Mark> check the
            following checkboxes:
            <ul>
              <li>Import New Users and Profile Updates</li>
              <li>Push New Users</li>
              <li>Push Profile Updates</li>
            </ul>
          </Text>
          <Text>
            Afterwards, select <Mark>HTTP Header</Mark> from the{' '}
            <Mark>Authentication Mode</Mark> dropdown.
          </Text>
          <Box>
            <Text mb={2} mt={2}>
              Copy and paste this Authorization <Mark>Bearer Token</Mark>:
            </Text>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text: `${installedPlugin.spec.scimBearerToken}`,
                },
              ]}
            />
          </Box>
          <Text mt={3}>
            Click <Mark>Save</Mark>
          </Text>
        </StyledBox>
        <StyledBox>
          <Text bold>Step 3: Configure SCIM Provisioning</Text>
          <Text>
            Staying on the <Mark>Provisioning</Mark> tab, go to the new{' '}
            <Mark>To App</Mark> Settings page and click <Mark>Edit</Mark>.
          </Text>
          <Text>
            Check the following checkboxes:
            <ul>
              <li>Create Users</li>
              <li>Update User Attributes</li>
              <li>Deactivate Users</li>
            </ul>
          </Text>
          <Text mt={3}>
            Click <Mark>Save</Mark>
          </Text>
        </StyledBox>
        <OutlineInfo>
          <Text bold>
            Please set up SCIM before proceeding—you will not be able to view
            the <Mark>Bearer Token</Mark> again!
          </Text>
        </OutlineInfo>
      </Flex>
      <Flex>
        <ButtonPrimary onClick={() => onFinish()}>Finish</ButtonPrimary>
      </Flex>
    </Box>
  );
}

export const StyledBox = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
  maxWidth: '800px',
})`
  background-color: ${props => props.theme.colors.spotBackground[0]};
`;
