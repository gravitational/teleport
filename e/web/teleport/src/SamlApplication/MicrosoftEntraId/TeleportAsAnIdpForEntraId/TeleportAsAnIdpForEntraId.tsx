import { useEffect, useState } from 'react';

import { Box, Flex, Indicator, Link, Mark, Text } from 'design';
import { Danger } from 'design/Alert';
import { IconTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { ButtonDownloadMetadataFile } from 'e-teleport/SamlApplication/components/ButtonDownloadMetadataFile';
import { useSamlApplication } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  StyledBox,
} from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';

/**
 * TeleportAsAnIdpForEntraId is a component to configure Entra ID
 * with Teleport SAML IdP metadata.
 */
export function TeleportAsAnIdpForEntraId() {
  const { prevStep, nextStep } = useDiscover();
  const {
    runFetchMetadataValues,
    fetchMetadataValuesAttempt,
    setGuidedToggle,
  } = useSamlApplication();

  setGuidedToggle(true);

  useEffect(() => {
    if (fetchMetadataValuesAttempt.status === '') {
      runFetchMetadataValues();
    }
  }, [runFetchMetadataValues, fetchMetadataValuesAttempt]);

  const [tenantId, setTenantId] = useState('');

  const handleNext = (v: Validator) => {
    if (!v.validate()) {
      return;
    }

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
            Configure Microsoft Entra ID with Teleport SAML IdP Metadata
          </Header>
          <HeaderSubtitle>
            You need to configure Microsoft Entra ID with Teleport SAML identity
            provider metadata.
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
                  <AddTenantId tenantId={tenantId} setTenantId={setTenantId} />
                </StyledBox>

                <ActionButtons
                  onProceed={() => handleNext(validator)}
                  onPrev={prevStep}
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
      <Text fontSize={2} bold>
        Prerequisites:
      </Text>
      <Flex gap={6} bg="levels.surface" borderRadius={2}>
        <ul>
          <li>
            <Flex alignItems="center">
              <Text>
                Microsoft Entra "Global Administrator" role that allows to
                create SAML IdP&nbsp;
                <Link
                  target="_blank"
                  href="https://learn.microsoft.com/en-us/entra/identity/role-based-access-control/permissions-reference#global-administrator"
                >
                  (docs)
                </Link>
                .
              </Text>
              <Flex ml={1}>
                <IconTooltip>Always assign least privileged roles.</IconTooltip>
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
  setTenantId: (e: string) => void;
}) {
  return (
    <>
      <Text mb={3}>
        In the Azure portal, from Azure services catalog menu, select&nbsp;
        <Mark>Microsoft Entra ID</Mark>. Under "Basic information" section, you
        will find Tenant ID. Copy and paste the Tenant ID value below.
      </Text>
      <FieldInput
        rule={requiredField('Tenant ID is required')}
        label="Tenant ID"
        toolTipContent="The value of Tenant ID will be used to derive SAML service provider entity ID and launch URL."
        value={tenantId}
        placeholder="e14205e2-0342-4d9b-9a00-60f7bc19648b"
        width="500px"
        onChange={e => setTenantId(e.target.value)}
      />
    </>
  );
}
