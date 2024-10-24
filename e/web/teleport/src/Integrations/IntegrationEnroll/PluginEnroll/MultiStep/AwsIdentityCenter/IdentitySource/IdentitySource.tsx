import { useState, useCallback } from 'react';
import { Box, ButtonPrimary, ButtonSecondary, Flex, Mark, Text } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { ButtonFileUpload } from 'shared/components/ButtonFileUpload';
import { StyledBox } from 'teleport/Discover/Shared';
import Validation, { Validator } from 'shared/components/Validation';
import { Danger } from 'design/Alert';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import { pluginsService } from 'e-teleport/services/plugins';
import { ButtonDownloadMetadataFile } from 'e-teleport/SamlApplication/components/ButtonDownloadMetadataFile';
import { usePlugin } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/usePlugin';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { PluginConfigAwsIc } from 'e-teleport/services/plugins/types';

export function AwsIcConfigureIdentitySource() {
  const [selectedFileContent, setSelectedFileContent] = useState<string>();
  const [showFileValidationError, setShowFileValidationError] =
    useState<boolean>(false);

  async function onFileSelect(f: File) {
    setShowFileValidationError(false);
    const fileData = await f.text();
    setSelectedFileContent(fileData);
  }

  const { formData, nextStep, prevStep, selectedPlugin } = usePlugin();

  const [samlServiceProviderName, setSAMLServiceProviderName] =
    useState<string>(PluginConfigAwsIc.PluginName);
  const [
    validateSAMLIdPServiceProviderAttempt,
    validateSAMLIdPServiceProvider,
  ] = useAsync(
    useCallback(async () => {
      const req = makeSAMLValidationRequest(
        samlServiceProviderName,
        selectedFileContent,
        selectedPlugin.type
      );
      return await pluginsService.validatePlugin(req);
    }, [samlServiceProviderName, selectedPlugin, selectedFileContent])
  );

  async function handleNext(v: Validator) {
    if (!v.validate()) {
      return;
    }
    if (!selectedFileContent) {
      setShowFileValidationError(true);
      return;
    }

    const [, err] = await validateSAMLIdPServiceProvider();
    if (err) {
      return;
    }
    formData.set(
      PluginConfigAwsIc.SamlServiceProviderMetadata,
      selectedFileContent
    );
    formData.set(
      PluginConfigAwsIc.SamlServiceProviderName,
      samlServiceProviderName
    );

    nextStep();
  }

  return (
    <Box minWidth="980px">
      <Header header="Configure Teleport as an Identity Source for Identity Center" />
      <Text>
        Update Identity Center with Teleport as an SAML identity provider and
        add identity center as an SAML service provider in Teleport.
      </Text>
      {validateSAMLIdPServiceProviderAttempt.status === 'error' && (
        <Box mt={3}>
          <Danger>{validateSAMLIdPServiceProviderAttempt.statusText}</Danger>
        </Box>
      )}
      <Flex
        mb={4}
        mt={4}
        flexDirection="column"
        alignItems="start"
        width="100%"
      >
        <Validation>
          {({ validator }) => (
            <>
              <StyledBox mb={4}>
                <Text bold mb={1}>
                  Step 1: Change identity source in AWS Identity Center
                </Text>
                <Text mb={2}>
                  In the AWS Identity Center console, navigate to{' '}
                  <Mark>Settings</Mark>. Under &nbsp;
                  <Mark>{`Settings > Identity source > Actions`}</Mark> menu,
                  select "Change identity source". Next, choose the "External
                  identity provider" option as an identity source.
                </Text>
              </StyledBox>

              <StyledBox mb={4}>
                <Text bold mb={1}>
                  Step 2: Add Identity Center SAML service provider metadata to
                  Teleport
                </Text>
                <Flex flexDirection="column" gap={3}>
                  First, enter the SAML service provider name for Identity
                  Center.
                  <FieldInput
                    rule={requiredField(
                      'Service provider name for AWS Identity Center is required'
                    )}
                    maxWidth={500}
                    label="SAML service provider name"
                    onChange={e => setSAMLServiceProviderName(e.target.value)}
                    value={samlServiceProviderName}
                    placeholder="Default SAML service provider name for Identity Center"
                  />
                  <Text>
                    Next, from the external identity provider configuration page
                    (Step 1), download the AWS Identity Center SAML service
                    provider metadata file by clicking on a button named{' '}
                    <Mark>Download metadata file</Mark> and upload it below.
                  </Text>
                  <ButtonFileUpload
                    text="Upload Identity Center Metadata File"
                    errorMessage="AWS Identity Center SAML service provider metadata file is required"
                    accept=".xml"
                    onFileSelect={onFileSelect}
                    showValidationError={showFileValidationError}
                    buttonSize="medium"
                    buttonIconSize="medium"
                    disabled={false}
                  />
                </Flex>
              </StyledBox>

              <StyledBox mb={2} width="980px">
                <Text bold mb={1}>
                  Step 3: Add Teleport SAML identity provider metadata to
                  Identity Center
                </Text>
                <Flex flexDirection="column" gap={3}>
                  <Text>First, download Teleport SAML IdP metadata file.</Text>
                  <ButtonDownloadMetadataFile />

                  <Text>
                    Now go back to the external identity provider configuration
                    page in the AWS Identity Center console again, and upload
                    the Teleport SAML IdP metadata file by clicking on a button
                    named <Mark>IdP SAML metadata</Mark>.
                  </Text>

                  <Text>
                    Once the file is uploaded, click <Mark>Next</Mark> button in
                    the Identity Center console to finish up configuring
                    external identity provider. After the configuration is
                    finished in the AWS, click on the <Mark>Next</Mark> button
                    below to configure SCIM integration.
                  </Text>
                </Flex>
              </StyledBox>
              <Flex mt={6} mb={5} gap={3}>
                <ButtonPrimary
                  onClick={() => handleNext(validator)}
                  disabled={!selectedFileContent}
                >
                  Next
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

function makeSAMLValidationRequest(
  samlServiceProviderName: string,
  samlServiceProviderMetadata: string,
  pluginType: string
) {
  const validationRequest = new FormData();
  validationRequest.set(
    PluginConfigAwsIc.SamlServiceProviderName,
    samlServiceProviderName
  );
  validationRequest.set('type', pluginType);
  validationRequest.set(
    PluginConfigAwsIc.SamlServiceProviderMetadata,
    samlServiceProviderMetadata
  );
  validationRequest.set(
    PluginConfigAwsIc.ResourceToValidate,
    PluginConfigAwsIc.ValidateSaml
  );
  return validationRequest;
}
