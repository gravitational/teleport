import React from 'react';

import { Text } from 'design';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
  UPDATE_NOTE,
} from 'e-teleport/SamlApplication/components/ConfigureServiceProvider';
import { useDiscover } from 'teleport/Discover/useDiscover';

/**
 * AddEntraSpToTeleport is a component to configure SAML service provider spec
 * for Entra ID preset.
 */
export function AddEntraSpToTeleport() {
  const { prevStep, nextStep, updateAgentMeta, agentMeta, isUpdateFlow } =
    useDiscover();

  const header: React.ReactNode = isUpdateFlow
    ? 'Update Microsoft Entra ID Service Provider'
    : 'Add Microsoft Entra ID SAML Service Provider To Teleport';

  const subtitle: React.ReactNode = (
    <Text>
      The Entity ID, ACS URL and attribute mapping values are derived from the
      Tenant ID value provided in the previous step.
      {isUpdateFlow && (
        <>
          <br /> {UPDATE_NOTE}
        </>
      )}
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
    />
  );
}
