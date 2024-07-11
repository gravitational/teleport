import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import { Text } from 'design';

import { useDiscover } from 'teleport/Discover/useDiscover';

import useTeleportE from 'e-teleport/useTeleportE';

import {
  ConfigureServiceProvider,
  AddMetadataGeneric,
  UPDATE_NOTE,
} from '../../shared/ConfigureServiceProvider';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const {
    prevStep,
    nextStep,
    updateAgentMeta,
    agentMeta,
    isUpdateFlow,
    resourceSpec,
  } = useDiscover();

  const upsertSP = (spConfig: CreateSamlIdpServiceProviderRequest) => {
    spConfig.preset = resourceSpec.samlMeta?.preset;
    run(() => idpService.upsertRequest(spConfig, isUpdateFlow));
  };

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
      attempt={attempt}
      upsertSP={upsertSP}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
      SpMetadataConfigComponent={AddMetadataGeneric}
      isUpdateFlow={isUpdateFlow}
    />
  );
}
