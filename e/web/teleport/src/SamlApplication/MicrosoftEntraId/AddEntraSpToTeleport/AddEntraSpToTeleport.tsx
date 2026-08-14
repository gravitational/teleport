import React from 'react';

import { Text } from 'design';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
} from 'e-teleport/SamlApplication/components/ConfigureServiceProvider';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

/**
 * AddEntraSpToTeleport is a component to configure SAML service provider spec
 * for Entra ID preset.
 */
export function AddEntraSpToTeleport() {
  const { prevStep, nextStep, updateAgentMeta, agentMeta, isUpdateFlow } =
    useDiscover();

  const header: React.ReactNode = isUpdateFlow
    ? 'Update Microsoft Entra External ID service provider'
    : 'Add Microsoft Entra External ID service provider To Teleport';

  const subtitle: React.ReactNode = (
    <Text>
      The Entity ID, ACS URL and default attribute is already configured for
      you.
    </Text>
  );

  return (
    <ConfigureServiceProvider
      header={header}
      subtitle={subtitle}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
      SpMetadataConfigComponent={AddMetadataGeneric}
      isUpdateFlow={isUpdateFlow}
      preset={SamlServiceProviderPreset.MicrosoftEntraId}
    />
  );
}
