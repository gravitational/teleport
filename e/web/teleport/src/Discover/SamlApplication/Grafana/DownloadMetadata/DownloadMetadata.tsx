import React from 'react';
import { Text, Link, ButtonPrimary, Mark } from 'design';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';

import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  HeaderSubtitle,
  Header,
  StepBox,
  ActionButtons,
} from 'teleport/Discover/Shared';

const idpMetadataUrl = cfg.baseUrl + '/enterprise/saml-idp/metadata';

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
  return (
    <>
      <Header>Download Teleport's Identity Provider Metadata</Header>
      <HeaderSubtitle>
        In order to use Teleport as a SAML Identity Provider for Grafana, you
        must configure Grafana to recognize Teleport's IdP metadata.
      </HeaderSubtitle>
      <StepOne />
      <StepTwo />
      <ActionButtons onProceed={nextStep} onPrev={prevStep} />
    </>
  );
}

function StepOne() {
  return (
    <StepBox mb={4}>
      <Text bold>Step 1</Text>
      <Text typography="subtitle1" mb={3}>
        Download Teleport's IdP metadata file.
      </Text>
      <ButtonPrimary as="a" href={idpMetadataUrl} mb={2}>
        Download Metadata
      </ButtonPrimary>
    </StepBox>
  );
}

function StepTwo() {
  return (
    <StepBox>
      <Text bold>Step 2</Text>
      <Text typography="subtitle1" mb={3}>
        Configure Grafana with the IdP metadata you just downloaded.
        <br />
      </Text>
      <Text mb={2}>
        From the Grafana host, add a <Mark>[auth.saml]</Mark> section to{' '}
        <Mark>grafana.ini</Mark> that points to the path of the IdP metadata
        file. Once you have saved the edited configuration, restart Grafana.
      </Text>
      <TextSelectCopyMulti
        bash={false}
        lines={[
          {
            text:
              `[auth.saml]\n` +
              `enabled = true\n` +
              `auto_login = false\n` +
              `idp_metadata_path = '/path/to/idp-metadata' # Path to Teleport's IdP metadata file which you just downloaded\n` +
              `private_key_path = '/path/to/certs/grafana-host-key.pem'\n` +
              `certificate_path = '/path/to/certs/grafana-host.pem'\n` +
              `assertion_attribute_name = uid\n` +
              `assertion_attribute_login = uid\n` +
              `assertion_attribute_email = uid\n` +
              `assertion_attribute_groups = eduPersonAffiliation\n` +
              `# To allow IdP-initiated login\n` +
              `allow_idp_initiated = true\n` +
              `relay_state = ""\n`,
          },
        ]}
      />
      <Text mt={1}>
        For more information on configuring Grafana for SAML, refer to the{' '}
        <Link
          target="_blank"
          href="https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/saml/"
        >
          Grafana SAML docs
        </Link>
        .
      </Text>
    </StepBox>
  );
}

type Props = {
  nextStep: () => void;
  prevStep: () => void;
};
