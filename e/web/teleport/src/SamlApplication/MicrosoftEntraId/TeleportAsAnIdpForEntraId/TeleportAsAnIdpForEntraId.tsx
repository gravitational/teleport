import { useEffect } from 'react';

import { Box, Flex, Indicator, Link, Mark, Text } from 'design';
import { Danger } from 'design/Alert';
import { IconTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { ButtonDownloadMetadataFile } from 'e-teleport/SamlApplication/components/ButtonDownloadMetadataFile';
import {
  checkDefaultAttributePerPreset,
  useSamlApplication,
} from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { microsoftEntraIdPresetSpec } from 'e-teleport/services/idp/types';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import { SamlMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

/**
 * TeleportAsAnIdpForEntraId is a component to configure Entra ID
 * with Teleport SAML IdP metadata.
 */
export function TeleportAsAnIdpForEntraId() {
  const { prevStep, nextStep, isUpdateFlow, agentMeta } = useDiscover();
  const {
    runFetchMetadataValues,
    fetchMetadataValuesAttempt,
    guidedToggle,
    setGuidedToggle,
    upsertRequest,
    setUpsertRequest,
    guidedConfig,
    setGuidedConfig,
  } = useSamlApplication();

  useEffect(() => {
    if (fetchMetadataValuesAttempt.status === '') {
      runFetchMetadataValues();
    }
  }, [runFetchMetadataValues, fetchMetadataValuesAttempt]);

  // value of agentMeta will be defined if user is coming to
  // this screen from update Discover flow or coming back from the
  // next screen. But it's value will be undefined if the user is coming
  // to this screen from the "Enroll New Resource" Discover screen.
  const samlMeta: SamlMeta = agentMeta
    ? agentMeta
    : defaultSamlMetaForMicrosoftEntraId;

  useEffect(() => {
    if (guidedToggle === null) {
      setGuidedToggle(true);
      setGuidedConfig({ samlMicrosoftEntraId: samlMeta.samlMicrosoftEntraId });
    }
  }, [guidedToggle, setGuidedToggle, setGuidedConfig, isUpdateFlow, samlMeta]);

  function setTenantId(e: React.ChangeEvent<HTMLInputElement>) {
    setGuidedConfig({
      ...guidedConfig,
      samlMicrosoftEntraId: {
        ...guidedConfig.samlMicrosoftEntraId,
        tenantId: e.target.value,
      },
    });
  }

  const handleNext = (v: Validator) => {
    if (!v.validate()) {
      return;
    }

    const entraPresetSpec = microsoftEntraIdPresetSpec(
      guidedConfig.samlMicrosoftEntraId.tenantId
    );
    // we don't set default attribute value in update flow as user may have previously
    // opted for manual configuration.
    const defaultAttribute =
      !isUpdateFlow &&
      checkDefaultAttributePerPreset(
        SamlServiceProviderPreset.MicrosoftEntraId,
        upsertRequest.attributeMapping
      )
        ? upsertRequest.attributeMapping
        : entraPresetSpec.attribute_mapping;

    setUpsertRequest({
      ...upsertRequest,
      entityID: entraPresetSpec.entity_id,
      acsURL: entraPresetSpec.acs_url,
      attributeMapping: defaultAttribute,
      launchURLs: entraPresetSpec.launch_urls,
    });

    nextStep();
  };

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
        <Box mb={4}>
          <Header>
            Configure Teleport as an identity provider for Microsoft Entra
            External ID
          </Header>
          <HeaderSubtitle>
            By configuring Teleport as an external SAML IdP for Microsoft Entra
            External ID, users can access Azure portal, CLI or any custom
            application set up in Azure by authenticating with Teleport.
          </HeaderSubtitle>

          <AzurePrerequisites />

          <StyledBox mb={4} mt={4}>
            <Text bold>Step 1: Configure Teleport as a SAML IdP</Text>
            <Text mb={3}>
              In the Azure portal, from Azure services catalog menu, select{' '}
              <Mark>External Identities</Mark>.<br />
              Under&nbsp;
              <Mark>{`All identity providers > Custom`}</Mark> menu, click "Add
              custom provider" button and choose the "SAML/WS-Fed" option.
            </Text>

            <Text mb={3}>
              In the "SAML/WS-Fed IdP" configuration page, provide a display
              name for the Teleport SAML IdP. <br />
              For the "Identity provider protocol" input, select "SAML"
              protocol. <br />
              For the "Domain name of federating IdP" input, enter your Teleport
              cluster name.
            </Text>

            <Text mb={2}>
              As a last step, in the "Select a method for populating metadata"
              option, select "Parse metadata file". <br />
              Now download the Teleport SAML IdP metadata file below and upload
              it to the Azure portal.
            </Text>
            <ButtonDownloadMetadataFile />
          </StyledBox>

          <Validation>
            {({ validator }) => (
              <>
                <StyledBox>
                  <Text bold>Step 2: Enter Microsoft Entra ID Tenant ID </Text>
                  <AddTenantId
                    tenantId={guidedConfig.samlMicrosoftEntraId.tenantId}
                    setTenantId={setTenantId}
                  />
                </StyledBox>

                <ActionButtons
                  onProceed={() => handleNext(validator)}
                  /**
                   * In an update flow, users should be prevent from navigating to
                   * the root Discover resource selection page.
                   */
                  onPrev={isUpdateFlow ? null : prevStep}
                />
              </>
            )}
          </Validation>
        </Box>
      );
  }
}

function AzurePrerequisites() {
  return (
    <>
      <Text fontSize={2} bold mb={1}>
        Prerequisites:
      </Text>
      <Flex gap={6} bg="levels.surface" borderRadius={2}>
        <ul>
          <li>
            <Flex alignItems="center">
              <Text>
                Microsoft Entra ID "Global Administrator" role&nbsp;
                <Link
                  target="_blank"
                  href="https://learn.microsoft.com/en-us/entra/identity/role-based-access-control/permissions-reference#global-administrator"
                >
                  (docs)
                </Link>
                .
              </Text>
              <Flex ml={1}>
                <IconTooltip>
                  You need permissions to create an external identity provider,
                  manage users and link billing subscription
                </IconTooltip>
              </Flex>
            </Flex>
          </li>
          <li>
            <Flex alignItems="center">
              <Text>
                Determine if DNS update is required&nbsp;
                <Link
                  target="_blank"
                  href="https://learn.microsoft.com/en-us/entra/external-id/direct-federation#step-1-determine-if-the-partner-needs-to-update-their-dns-text-records"
                >
                  (docs)
                </Link>
                .
              </Text>
            </Flex>
          </li>
        </ul>
      </Flex>
    </>
  );
}

function AddTenantId({
  tenantId,
  setTenantId,
}: {
  tenantId: string;
  setTenantId: (e: React.ChangeEvent<HTMLInputElement>) => void;
}) {
  return (
    <>
      <Text mb={3}>
        In the Azure portal, from Azure services catalog menu, select&nbsp;
        <Mark>Microsoft Entra ID</Mark>. Under "Basic information" section, you
        will find Tenant ID. Copy and paste the Tenant ID value below.
      </Text>
      <FieldInput
        name="tenantId"
        // TODO(sshah): Add tenant ID validation. The length and format
        // looks uuid v4 but need to double confirm.
        rule={requiredField('Tenant ID is required')}
        label="Tenant ID"
        toolTipContent="The value of Tenant ID will be used to derive SAML service provider entity ID and launch URL."
        value={tenantId}
        placeholder="e14205e2-0342-4d9b-9a00-60f7bc19648b"
        width="500px"
        onChange={e => setTenantId(e)}
      />
    </>
  );
}

const defaultSamlMetaForMicrosoftEntraId: SamlMeta = {
  samlMicrosoftEntraId: {
    tenantId: '',
  },
};
