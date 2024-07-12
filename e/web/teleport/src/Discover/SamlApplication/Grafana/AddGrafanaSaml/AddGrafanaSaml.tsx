import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Mark } from 'design/Mark';

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
    ? 'Update Grafana Service Provider'
    : 'Add Grafana SAML Configuration to Teleport';

  const subtitle: React.ReactNode = (
    <>
      Enter a name for the integration and paste your Grafana host's entity
      descriptor XML content. <br />
      You can find your entity descriptor at{' '}
      <Mark>https://{`<your-grafana-url>`}/saml/metadata</Mark>.
      {isUpdateFlow && (
        <>
          <br /> {UPDATE_NOTE}
        </>
      )}
    </>
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
