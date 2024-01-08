import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';

import { Mark } from 'teleport/Discover/Shared';
import { useDiscover } from 'teleport/Discover/useDiscover';

import useTeleportE from 'e-teleport/useTeleportE';

import { ServiceProvider } from '../../SamlApp/AddServiceProvider';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const { prevStep, nextStep, updateAgentMeta, agentMeta } = useDiscover();

  const createSP = (spConfig: CreateSamlIdpServiceProviderRequest) => {
    run(() => idpService.createSamlIdpServiceProvider(spConfig));
  };

  const header: React.ReactNode = 'Add Grafana SAML Configuration to Teleport';
  const subtitle: React.ReactNode = (
    <>
      Enter a name for the integration and paste your Grafana host's entity
      descriptor XML content. <br />
      You can find your entity descriptor at{' '}
      <Mark>https://{`<your-grafana-url>`}/saml/metadata</Mark>.
    </>
  );

  return (
    <ServiceProvider
      header={header}
      subtitle={subtitle}
      attempt={attempt}
      createSP={createSP}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
    />
  );
}
