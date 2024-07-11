import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Box, Link, Text } from 'design';

import {
  useDiscover,
  AgentMeta,
  SamlMeta,
} from 'teleport/Discover/useDiscover';

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
    resourceSpec,
    isUpdateFlow,
  } = useDiscover();
  const gcpWorkforceMeta: Extract<SamlMeta, AgentMeta> = agentMeta;

  const upsertSP = (spConfig: CreateSamlIdpServiceProviderRequest) => {
    spConfig.preset = resourceSpec.samlMeta?.preset;
    run(() => idpService.upsertRequest(spConfig, isUpdateFlow));
  };

  const header: React.ReactNode = isUpdateFlow
    ? 'Update Workforce Pool'
    : 'Add Workforce Pool To Teleport';

  const subtitle: React.ReactNode = gcpWorkforceMeta.samlGcpWorkforce
    ?.isAutoConfig ? (
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
        for instruction's on how to obtain the Entity ID and ACS URL.
      </Text>
    </Box>
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
      resourceSpec={resourceSpec}
      isUpdateFlow={isUpdateFlow}
    />
  );
}
