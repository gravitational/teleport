import React from 'react';

import { Text } from 'design';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
  UPDATE_NOTE,
} from 'e-teleport/SamlApplication/components/ConfigureServiceProvider';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

export function Container() {
  const { prevStep, nextStep, updateAgentMeta, agentMeta, isUpdateFlow } =
    useDiscover();

  const header: React.ReactNode = isUpdateFlow
    ? 'Update Service Provider'
    : 'Add Service Provider To Teleport';

  const subtitle: React.ReactNode = (
    <Text>
      Please refer to your Service Provider's documentation for instructions on
      how to obtain the Entity ID and ACS URL.
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
      preset={SamlServiceProviderPreset.Unspecified}
    />
  );
}
