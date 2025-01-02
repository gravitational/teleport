import { useCallback, useState } from 'react';

import {
  Box,
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Mark,
  Text,
} from 'design';
import { Danger, Success } from 'design/Alert';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { usePlugin } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { pluginsService } from 'e-teleport/services/plugins';
import {
  PluginConfigAwsIc,
  PluginConfigBase,
} from 'e-teleport/services/plugins/types';
import { StyledBox } from 'teleport/Discover/Shared';
import { getXCSRFToken } from 'teleport/services/api';

import { requiredHttpsUrl } from '../rules';

export function AwsIcConfigureScim() {
  const { nextStep, prevStep, formData, selectedPlugin, setInstalledPlugin } =
    usePlugin();
  const [baseUrl, setBaseUrl] = useState('');
  const [accessToken, setAccessToken] = useState('');

  const [createPluginAttempt, createPlugin] = useAsync(
    useCallback(async () => {
      const resp = await pluginsService.createPlugin(formData);
      setInstalledPlugin(resp);
    }, [formData, setInstalledPlugin])
  );

  const [validateAttempt, runScimValidation] = useAsync(
    useCallback(async () => {
      const req = makeSCIMValidationRequest(
        baseUrl.trim(),
        accessToken.trim(),
        selectedPlugin.type
      );
      return await pluginsService.validatePlugin(req);
    }, [baseUrl, accessToken, selectedPlugin])
  );

  const validateScimCredential = async (v: Validator) => {
    if (!v.validate()) {
      return;
    }
    runScimValidation();
  };

  const handleFinish = async (v: Validator) => {
    if (!v.validate()) {
      return;
    }
    runScimValidation();
    formData.set(PluginConfigAwsIc.ScimBaseURL, baseUrl.trim());
    formData.set(PluginConfigAwsIc.ScimAccessToken, accessToken.trim());
    formData.set(PluginConfigBase.CSRFToken, getXCSRFToken());
    formData.set(PluginConfigBase.Name, selectedPlugin.type);
    formData.set(PluginConfigBase.Type, selectedPlugin.type);
    const [, err] = await createPlugin();
    if (err) {
      return;
    }

    nextStep();
  };

  function AttemptBanner() {
    switch (true) {
      case createPluginAttempt.status === 'error' ||
        validateAttempt.status === 'error':
        return (
          <Danger>
            {' '}
            {createPluginAttempt.statusText || validateAttempt.statusText}
          </Danger>
        );
      case validateAttempt.status === 'success':
        return <Success>SCIM credential is valid.</Success>;
      default:
        return null;
    }
  }

  return (
    <Box width="800px">
      <Box mb={3}>
        <Header header="Set Up SCIM" />
        <Text>
          With SCIM (System for Cross-domain Identity Management) integration,
          Teleport will provision user and user groups to the AWS IAM Identity
          Center.
        </Text>
      </Box>
      <Box mt={3}>
        <AttemptBanner />
      </Box>
      <Flex mb={1} mt={4} flexDirection="column" gap={4} width="100%">
        <Validation>
          {({ validator }) => (
            <>
              <StyledBox>
                <Text bold>Step 1: Enable SCIM in AWS IAM Identity Center</Text>
                <Text>
                  In the AWS IAM Identity Center console, navigate to{' '}
                  <Mark>Settings</Mark> page. Locate an admonition titled
                  "Automatic provisioning" and click <Mark>Enable</Mark> button
                  available inside admonition. Once enabled, copy the SCIM
                  endpoint and Access token values from AWS IAM Identity Center
                  and paste those value below.
                </Text>
                <Flex flexDirection="column" maxWidth={500} mt={3}>
                  <FieldInput
                    rule={requiredHttpsUrl}
                    label="AWS IAM Identity Center SCIM endpoint"
                    onChange={e => setBaseUrl(e.target.value)}
                    value={baseUrl}
                    placeholder="https://scim.amazonaws.com/../scim/v2"
                  />
                  <FieldInput
                    rule={requiredField('Access token is required')}
                    label="AWS IAM Identity Center SCIM Access token"
                    type="password"
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
                <ButtonBorder
                  size="medium"
                  px={3}
                  onClick={() => validateScimCredential(validator)}
                >
                  Test SCIM
                </ButtonBorder>
              </StyledBox>
              <Flex mt={3} gap={4}>
                <ButtonPrimary onClick={() => handleFinish(validator)}>
                  Install plugin
                </ButtonPrimary>
                <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
              </Flex>
            </>
          )}
        </Validation>
      </Flex>
    </Box>
  );
}

function makeSCIMValidationRequest(
  baseUrl: string,
  accessToken: string,
  pluginType: string
) {
  const validationRequest = new FormData();
  validationRequest.set('type', pluginType);
  validationRequest.set(PluginConfigAwsIc.ScimBaseURL, baseUrl);
  validationRequest.set(PluginConfigAwsIc.ScimAccessToken, accessToken);
  validationRequest.set(
    PluginConfigAwsIc.ResourceToValidate,
    PluginConfigAwsIc.ValidateScim
  );
  return validationRequest;
}
