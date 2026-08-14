import React from 'react';

import { Mark } from 'design/Mark';

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
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
      SpMetadataConfigComponent={AddMetadataGeneric}
      preset={SamlServiceProviderPreset.Unspecified}
      isUpdateFlow={isUpdateFlow}
    />
  );
}
