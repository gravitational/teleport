import { useState } from 'react';
import { Box, ButtonPrimary, ButtonSecondary, Flex, Mark, Text } from 'design';
import { ButtonFileUpload } from 'shared/components/ButtonFileUpload';
import { StyledBox } from 'teleport/Discover/Shared';

import { ButtonDownloadMetadataFile } from 'e-teleport/SamlApplication/components/ButtonDownloadMetadataFile';

import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';

export function AwsIcConfigureIdentitySource() {
  const [selectedFile, setSelectedFile] = useState<File>();
  const [showFileValidationError, setShowFileValidationError] =
    useState<boolean>(false);

  function onFileSelect(f: File) {
    setShowFileValidationError(false);
    setSelectedFile(f);
  }

  function handleNext() {
    if (!selectedFile?.name) {
      setShowFileValidationError(!selectedFile?.name);
      return;
    }
  }

  return (
    <Box minWidth="980px">
      <Header header="Configure Teleport as an Identity Source for Identity Center" />
      <Text>
        Update Identity Center with Teleport as an SAML identity provider and
        add identity center as an SAML service provider in Teleport.
      </Text>
      <Flex
        mb={4}
        mt={4}
        flexDirection="column"
        alignItems="start"
        width="100%"
      >
        <StyledBox mb={4} width="980px">
          <Text bold mb={1}>
            Step 1: Change identity source in AWS Identity Center
          </Text>
          <Text mb={2}>
            In the AWS Identity Center console, navigate to{' '}
            <Mark>Settings</Mark>. Under &nbsp;
            <Mark>{`Settings > Identity source > Actions`}</Mark> menu, select
            "Change identity source". Next, choose the "External identity
            provider" option as an identity source.
          </Text>
        </StyledBox>

        <StyledBox mb={4} width="980px">
          <Text bold mb={1}>
            Step 2: Add Identity Center SAML service provider metadata to
            Teleport
          </Text>
          <Text mb={2}>
            From the external identity provider configuration page(Step 1),
            download the AWS Identity Center SAML service provider metadata file
            by clicking on a button named <Mark>Download metadata file</Mark>.
          </Text>

          <Text mb={3}>
            Now upload the SAML metadata file you just downloaded from AWS
            Identity Center.
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
        </StyledBox>

        <StyledBox mb={2} width="980px">
          <Text bold mb={1}>
            Step 3: Add Teleport SAML identity provider metadata to Identity
            Center
          </Text>

          <Text mb={3}>First, download Teleport SAML IdP metadata file.</Text>
          <ButtonDownloadMetadataFile />

          <Text mb={1} mt={2}>
            Now go back to the external identity provider configuration page in
            the AWS Identity Center console again, and upload the Teleport SAML
            IdP metadata file by clicking on a button named{' '}
            <Mark>IdP SAML metadata</Mark>.
          </Text>

          <Text mb={1} mt={2}>
            Once the file is uploaded, click <Mark>Next</Mark> button in the
            Identity Center console to finish up configuring external identity
            provider. After the configuration is finished in the AWS, click on
            the <Mark>Next</Mark> button below to configure SCIM integration.
          </Text>

          {/* TODO(sshah): read integration name from step 1 and display the
          SAML service provider name which we will auto-generate here.  */}
        </StyledBox>
        <Flex mt={6} mb={5} gap={3}>
          {/* TODO(sshah): server-side validation will be added for uploaded SAML entity descriptor file */}
          <ButtonPrimary
            onClick={() => handleNext()}
            disabled={!selectedFile?.name}
          >
            Next
          </ButtonPrimary>
          <ButtonSecondary>Back</ButtonSecondary>
        </Flex>
      </Flex>
    </Box>
  );
}
