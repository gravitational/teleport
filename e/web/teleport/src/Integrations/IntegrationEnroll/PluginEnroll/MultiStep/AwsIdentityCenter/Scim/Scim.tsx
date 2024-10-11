import { useState } from 'react';
import { Box, ButtonBorder, ButtonPrimary, Flex, Mark, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { StyledBox } from 'teleport/Discover/Shared';

import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';

import { requiredHttpsUrl } from '../rules';

export function AwsIcConfigureScim() {
  const [baseUrl, setBaseUrl] = useState('');
  const [accessToken, setAccessToken] = useState('');

  const handleFinish = (v: Validator) => {
    if (!v.validate()) {
      return;
    }
    // TODO(sshah): send create plugin request on finish.
  };

  return (
    <Box width="800px">
      <Box mb={3}>
        <Header header="Set Up SCIM" />
        <Text>
          With SCIM (System for Cross-domain Identity Management) integration,
          Teleport will provision user and user groups to the Identity Center.
        </Text>
      </Box>
      <Flex mb={1} mt={4} flexDirection="column" gap={4} width="100%">
        <Validation>
          {({ validator }) => (
            <>
              <StyledBox>
                <Text bold>Step 1: Enable SCIM in Identity Center</Text>
                <Text>
                  In the AWS Identity Center console, navigate to{' '}
                  <Mark>Settings</Mark> page. Locate an admonition titled
                  "Automatic provisioning" and click <Mark>Enable</Mark> button
                  available inside admonition. Once enabled, copy the SCIM
                  endpoint and Access token values from Identity Center and
                  paste those value below.
                </Text>
                <Flex flexDirection="column" maxWidth={500} mt={3}>
                  <FieldInput
                    rule={requiredHttpsUrl}
                    label="Identity Center SCIM endpoint"
                    onChange={e => setBaseUrl(e.target.value)}
                    value={baseUrl}
                    placeholder="https://scim.amazonaws.com/../scim/v2"
                  />
                  <FieldInput
                    rule={requiredField('Access token is required')}
                    label="Identity Center SCIM Access token"
                    onChange={e => setAccessToken(e.target.value)}
                    value={accessToken}
                    placeholder="access_token"
                  />
                </Flex>
              </StyledBox>
              <StyledBox>
                <Text bold>Step 2: Test SCIM Connection</Text>
                <Text mb={2}>
                  Click on the Test button below to verify SCIM credential is
                  set up correctly.
                </Text>
                {/* TODO(sshah): do a real server side SCIM credential validation */}
                <ButtonBorder
                  size="medium"
                  px={3}
                  onClick={() => handleFinish(validator)}
                >
                  Test SCIM
                </ButtonBorder>
              </StyledBox>
              <Flex mt={3}>
                <ButtonPrimary onClick={() => handleFinish(validator)}>
                  Install plugin
                </ButtonPrimary>
              </Flex>
            </>
          )}
        </Validation>
      </Flex>
    </Box>
  );
}
