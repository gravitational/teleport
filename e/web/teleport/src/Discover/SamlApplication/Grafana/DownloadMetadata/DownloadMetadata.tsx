import { ButtonPrimary, H3, Link, Mark, Subtitle3 } from 'design';
import { P } from 'design/Text/Text';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import cfg from 'teleport/config';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StepBox,
} from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';

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
      <header>
        <H3>Step 1</H3>
        <Subtitle3 mb={3}>Download Teleport's IdP metadata file</Subtitle3>
      </header>
      <ButtonPrimary as="a" href={idpMetadataUrl} mb={2}>
        Download Metadata
      </ButtonPrimary>
    </StepBox>
  );
}

function StepTwo() {
  return (
    <StepBox>
      <header>
        <H3>Step 2</H3>
        <Subtitle3 typography="subtitle3" mb={3}>
          Configure Grafana with the IdP metadata you just downloaded
        </Subtitle3>
      </header>
      <P mb={3}>
        From the Grafana host, add a <Mark>[auth.saml]</Mark> section to{' '}
        <Mark>grafana.ini</Mark> that points to the path of the IdP metadata
        file. Once you have saved the edited configuration, restart Grafana.
      </P>
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
      <P mt={3}>
        For more information on configuring Grafana for SAML, refer to the{' '}
        <Link
          target="_blank"
          href="https://grafana.com/docs/grafana/latest/setup-grafana/configure-security/configure-authentication/saml/"
        >
          Grafana SAML docs
        </Link>
        .
      </P>
    </StepBox>
  );
}

type Props = {
  nextStep: () => void;
  prevStep: () => void;
};
