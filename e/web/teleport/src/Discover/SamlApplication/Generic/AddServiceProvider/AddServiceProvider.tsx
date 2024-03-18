import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import { useDiscover } from 'teleport/Discover/useDiscover';

import useTeleportE from 'e-teleport/useTeleportE';

import {
  ConfigureServiceProvider,
  AddMetadataGeneric,
} from '../../shared/ConfigureServiceProvider';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const { prevStep, nextStep, updateAgentMeta, agentMeta } = useDiscover();

  const createSP = (spConfig: CreateSamlIdpServiceProviderRequest) => {
    run(() => idpService.createSamlIdpServiceProvider(spConfig));
  };

  const header: React.ReactNode = 'Add Service Provider To Teleport';
  const subtitle: React.ReactNode =
    "Please refer to your Service Provider's documentation for instruction's on how to obtain the Entity ID and ACS URL.";

  return (
    <ConfigureServiceProvider
      header={header}
      subtitle={subtitle}
      attempt={attempt}
      createSP={createSP}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
      SpMetadataConfigComponent={AddMetadataGeneric}
    />
  );
}
