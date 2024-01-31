import React, { useState, useEffect } from 'react';
import { Box, ButtonBorder, Flex, LabelInput, Text } from 'design';
import { Danger } from 'design/Alert';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';
import cfg from 'teleport/config';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import { useAttemptNext } from 'shared/hooks';

import useTeleportE from 'e-teleport/useTeleportE';

import type { SAMLIdPMetadataResponse } from 'e-teleport/services/idp/types';

const idpMetadataUrl = cfg.baseUrl + '/enterprise/saml-idp/metadata';

export function Container() {
  const { prevStep, nextStep } = useDiscover();

  return <DownloadMetadata prevStep={prevStep} nextStep={nextStep} />;
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
}: ConfigureSPProps) {
  return (
    <StyledBox mb={4} padding={6}>
      <Text bold>Teleport IdP Metadata</Text>

      <Flex alignItems="baseline" gap={4} mb={4}>
        <Text typography="subtitle1">
          Use the Teleport IdP metadata values shown below to configure service
          provider. You may also download the metadata file if you need more
          control over configuration or if the service provider requires to
          upload the metadata file.
        </Text>
        <ButtonBorder width="300px" as="a" href={idpMetadataUrl} size="medium">
          Download IdP Metadata
        </ButtonBorder>
      </Flex>

      <Box mb={4}>
        <LabelInput>Entity ID:</LabelInput>
        <TextSelectCopyMulti
          bash={false}
          lines={[{ text: samlIdPMetadata.entityID }]}
        />
      </Box>
      <Box mb={4}>
        <LabelInput>SSO URL:</LabelInput>
        <TextSelectCopyMulti
          bash={false}
          lines={[{ text: samlIdPMetadata.ssoURL }]}
        />
      </Box>
      <Box mb={4}>
        <LabelInput>X.509 Certificate:</LabelInput>
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

type ConfigureSPProps = {
  samlIdPMetadata: SAMLIdPMetadataResponse;
};
