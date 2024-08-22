import React, { useState, useEffect } from 'react';
import { Box, Flex, H3, LabelInput, Text } from 'design';
import { Danger } from 'design/Alert';
import { ToolTipInfo } from 'shared/components/ToolTip';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { useAttemptNext } from 'shared/hooks';
import { P } from 'design/Text/Text';

import useTeleportE from 'e-teleport/useTeleportE';

import { ButtonDownloadMetadataFile } from 'e-teleport/Discover/SamlApplication/shared/ButtonDownloadMetadataFile';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';

export function Container() {
  const { prevStep, nextStep, isUpdateFlow } = useDiscover();

  return (
    <DownloadMetadata
      /**
       * In an update flow, user's should be prevent from navigating to
       * the root Discover resource selection page.
       */
      prevStep={isUpdateFlow ? null : prevStep}
      nextStep={nextStep}
    />
  );
}

export function DownloadMetadata({ prevStep, nextStep }: Props) {
  const { idpService } = useTeleportE();
  const { attempt, setAttempt } = useAttemptNext('processing');
  const [samlIdPMetadata, setSAMLIdPMetadata] =
    useState<SAMLIdPMetadataResponse>({
      entityID: '',
      ssoURL: '',
      x509PEM: '',
    });

  useEffect(() => {
    // This request will only succeed when the web ui is directly served by the
    // Teleport proxy.
    idpService
      .getIdPMetadataValues()
      .then(resp => {
        setAttempt({ status: 'success' });
        setSAMLIdPMetadata(resp);
      })
      .catch((err: Error) => {
        setAttempt({ status: 'failed', statusText: err.message });
      });
  }, []);

  return (
    <>
      <Header>
        Configure Service Provider with Teleport's Identity Provider Metadata
      </Header>
      <HeaderSubtitle>
        In order to use Teleport as an Identity Provider for your SAML
        application, you must configure your service provider to recognize
        Teleport's IdP metadata.
      </HeaderSubtitle>
      {attempt.status === 'failed' && <Danger>{attempt.statusText}</Danger>}
      <ConfigureServiceProvider samlIdPMetadata={samlIdPMetadata} />
      <ActionButtons onProceed={nextStep} onPrev={prevStep} />
    </>
  );
}

export function ConfigureServiceProvider({
  samlIdPMetadata,
}: {
  samlIdPMetadata: SAMLIdPMetadataResponse;
}) {
  return (
    <StyledBox mb={4}>
      <Flex alignItems="center" justifyContent="space-between" mb={3}>
        <H3>Teleport SAML IdP Metadata</H3>
        <ButtonDownloadMetadataFile />
      </Flex>

      <P mb={4}>
        Your service provider requires the following Teleport SAML IdP metadata
        values to trust Teleport as an SAML identity provider. You can copy and
        paste these values or download the metadata file and upload it to your
        service provider.
      </P>

      <Box mb={4}>
        <LabelInput>
          <Flex alignItems="center">
            <Text mr={1}> IdP Entity ID / Issuer URL</Text>
            <ToolTipInfo>
              IdP Entity ID is a unique identifier of Teleport SAML identity
              provider, which is responsible for signing the SAML assertion.
              This is also commonly referred to as an Issuer URL.
            </ToolTipInfo>
          </Flex>
        </LabelInput>
        <TextSelectCopyMulti
          bash={false}
          lines={[{ text: samlIdPMetadata.entityID }]}
        />
      </Box>
      <Box mb={4}>
        <LabelInput>
          <Flex alignItems="center">
            <Text mr={1}> IdP SSO URL</Text>
            <ToolTipInfo>
              IdP Single Sign-on (SSO) URL is a URL location (usually a Teleport
              proxy endpoint) where the service provider will redirect users to
              authenticate with the IdP.
            </ToolTipInfo>
          </Flex>
        </LabelInput>
        <TextSelectCopyMulti
          bash={false}
          lines={[{ text: samlIdPMetadata.ssoURL }]}
        />
      </Box>
      <Box mb={4}>
        <LabelInput>
          <Flex alignItems="center">
            <Text mr={1}> IdP X.509 Certificate</Text>
            <ToolTipInfo>
              Service provider verifies that the SAML assertion is signed by
              Teleport SAML IdP using this IdP X.509 Certificate.
            </ToolTipInfo>
          </Flex>
        </LabelInput>
        <TextSelectCopyMulti
          bash={false}
          saveContent={{ save: true, filename: 'Teleport-SAML-IDP-X509.pem' }}
          maxHeight="300px"
          lines={[{ text: samlIdPMetadata.x509PEM }]}
        />
      </Box>
    </StyledBox>
  );
}

type Props = {
  nextStep: () => void;
  prevStep: () => void;
};
