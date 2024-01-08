import React from 'react';
import { Text, ButtonPrimary } from 'design';

import cfg from 'teleport/config';
import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  HeaderSubtitle,
  Header,
  ActionButtons,
  StyledBox,
} from 'teleport/Discover/Shared';

const idpMetadataUrl = cfg.baseUrl + '/enterprise/saml-idp/metadata';

export function Container() {
  const { prevStep, nextStep } = useDiscover();

  return <DownloadMetadata prevStep={prevStep} nextStep={nextStep} />;
}

export function DownloadMetadata({ prevStep, nextStep }: Props) {
  return (
    <>
      <Header>Download Teleport's Identity Provider Metadata</Header>
      <HeaderSubtitle>
        In order to use Teleport as an Identity Provider for your SAML
        application, you must configure your service provider to recognize
        Teleport's IdP metadata.
      </HeaderSubtitle>
      <StepOne />
      <StepTwo />
      <ActionButtons onProceed={nextStep} onPrev={prevStep} />
    </>
  );
}

function StepOne() {
  return (
    <StyledBox mb={4} padding={6}>
      <Text bold>Step 1</Text>
      <Text typography="subtitle1" mb={3}>
        Download Teleport's IdP metadata file.
      </Text>
      <ButtonPrimary as="a" href={idpMetadataUrl} mb={2}>
        Download Metadata
      </ButtonPrimary>
    </StyledBox>
  );
}

function StepTwo() {
  return (
    <StyledBox>
      <Text bold>Step 2</Text>
      <Text typography="subtitle1" mb={2}>
        Configure your service provider with the IdP metadata you just
        downloaded. Refer to your service provider's documentation for steps on
        how to do this. Once you're done with this step, click on the 'Next'
        button below.
        <br />
      </Text>
    </StyledBox>
  );
}

type Props = {
  nextStep: () => void;
  prevStep: () => void;
};
