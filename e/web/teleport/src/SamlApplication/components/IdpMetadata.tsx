import { useEffect } from 'react';

import { Box, Flex, H3, Indicator, LabelInput, Text } from 'design';
import { Danger } from 'design/Alert';
import { P } from 'design/Text/Text';
import { IconTooltip } from 'design/Tooltip';

import { ButtonDownloadMetadataFile } from 'e-teleport/SamlApplication/components/ButtonDownloadMetadataFile';
import { useSamlApplication } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { StyledBox } from 'teleport/Discover/Shared';

/**
 * IdpMetadata renders UI to copy or download SAML IdP metadata.
 */
export function IdpMetadata() {
  const { runFetchMetadataValues, fetchMetadataValuesAttempt } =
    useSamlApplication();

  useEffect(() => {
    if (fetchMetadataValuesAttempt.status === '') {
      runFetchMetadataValues();
    }
  }, [runFetchMetadataValues, fetchMetadataValuesAttempt]);

  switch (fetchMetadataValuesAttempt.status) {
    case '':
    case 'processing':
      return (
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      );
    case 'error':
      return <Danger>Failed to fetch SAML IdP metadata</Danger>;
    case 'success':
      return (
        <StyledBox mb={4}>
          <Flex alignItems="center" justifyContent="space-between" mb={3}>
            <H3>Teleport SAML IdP Metadata</H3>
            <ButtonDownloadMetadataFile />
          </Flex>

          <P mb={4}>
            Your service provider requires the following Teleport SAML IdP
            metadata values to trust Teleport as an SAML identity provider. You
            can copy and paste these values or download the metadata file and
            upload it to your service provider.
          </P>

          <Box mb={4}>
            <LabelInput>
              <Flex alignItems="center">
                <Text mr={1}> IdP Entity ID / Issuer URL</Text>
                <IconTooltip>
                  IdP Entity ID is a unique identifier of Teleport SAML identity
                  provider, which is responsible for signing the SAML assertion.
                  This is also commonly referred to as an Issuer URL.
                </IconTooltip>
              </Flex>
            </LabelInput>
            <TextSelectCopyMulti
              bash={false}
              lines={[{ text: fetchMetadataValuesAttempt.data.entityID }]}
            />
          </Box>
          <Box mb={4}>
            <LabelInput>
              <Flex alignItems="center">
                <Text mr={1}> IdP SSO URL</Text>
                <IconTooltip>
                  IdP Single Sign-on (SSO) URL is a URL location (usually a
                  Teleport proxy endpoint) where the service provider will
                  redirect users to authenticate with the IdP.
                </IconTooltip>
              </Flex>
            </LabelInput>
            <TextSelectCopyMulti
              bash={false}
              lines={[{ text: fetchMetadataValuesAttempt.data.ssoURL }]}
            />
          </Box>
          <Box mb={4}>
            <LabelInput>
              <Flex alignItems="center">
                <Text mr={1}> IdP X.509 Certificate</Text>
                <IconTooltip>
                  Service provider verifies that the SAML assertion is signed by
                  Teleport SAML IdP using this IdP X.509 Certificate.
                </IconTooltip>
              </Flex>
            </LabelInput>
            <TextSelectCopyMulti
              bash={false}
              saveContent={{
                save: true,
                filename: 'Teleport-SAML-IDP-X509.pem',
              }}
              maxHeight="300px"
              lines={[{ text: fetchMetadataValuesAttempt.data.x509PEM }]}
            />
          </Box>
        </StyledBox>
      );
  }
}
