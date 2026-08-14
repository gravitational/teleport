import React from 'react';

import { Box, Link, Text } from 'design';

import {
  AddMetadataGeneric,
  ConfigureServiceProvider,
  UPDATE_NOTE,
} from 'e-teleport/SamlApplication/components/ConfigureServiceProvider';
import { useSamlApplication } from 'e-teleport/SamlApplication/hooks/useSamlApplication';
import { useDiscover } from 'teleport/Discover/useDiscover';
import { SamlServiceProviderPreset } from 'teleport/services/samlidp/types';

export function Container() {
  const { prevStep, nextStep, updateAgentMeta, agentMeta, isUpdateFlow } =
    useDiscover();
  const { guidedConfig } = useSamlApplication();

  const header: React.ReactNode = isUpdateFlow
    ? 'Update Workforce Pool'
    : 'Add Workforce Pool To Teleport';

  const subtitle: React.ReactNode = guidedConfig ? (
    <Text>
      The fields below use the values from the GCP configuration provided in the
      previous step. If you update the workforce <br />
      pool name, pool provider name, or attribute mapping values in GCP, you
      must also update the Entity ID, ACS URL, <br />
      or attribute mapping fields in the SAML IdP service provider spec,
      respectively.
      {isUpdateFlow && (
        <>
          <br />
          <br /> {UPDATE_NOTE}
        </>
      )}
    </Text>
  ) : (
    <Box>
      <Text>
        Please refer to{' '}
        <Link
          target="_blank"
          href="https://cloud.google.com/iam/docs/configuring-workforce-identity-federation#create-saml-provider"
        >
          GCP documentation
        </Link>{' '}
        for instructions on how to obtain the Entity ID and ACS URL.
        {isUpdateFlow && (
          <>
            <br />
            <br /> {UPDATE_NOTE}
          </>
        )}
      </Text>
    </Box>
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
      preset={SamlServiceProviderPreset.GcpWorkforce}
      isUpdateFlow={isUpdateFlow}
    />
  );
}
