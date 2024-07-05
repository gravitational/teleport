import React from 'react';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Box, Link, Text } from 'design';

import { useDiscover } from 'teleport/Discover/useDiscover';

import useTeleportE from 'e-teleport/useTeleportE';

import {
  ConfigureServiceProvider,
  AddMetadataGeneric,
} from '../../shared/ConfigureServiceProvider';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';

import type { SamlGcpWorkforce } from 'teleport/services/samlidp/types';

export function Container() {
  const { idpService } = useTeleportE();
  const { attempt, run } = useAttempt('');
  const { prevStep, nextStep, updateAgentMeta, agentMeta, resourceSpec } =
    useDiscover();
  const gcpWorkforceMeta = agentMeta as SamlGcpWorkforce;

  const createSP = (spConfig: CreateSamlIdpServiceProviderRequest) => {
    spConfig.preset = resourceSpec.samlMeta?.preset;
    run(() => idpService.createSamlIdpServiceProvider(spConfig));
  };

  const header: React.ReactNode = 'Add Workforce Pool To Teleport';
  const subtitle: React.ReactNode = gcpWorkforceMeta?.isAutoConfig ? (
    <Text>
      The fields below use the values from the GCP configuration provided in the
      previous step. If you update the workforce <br />
      pool name, pool provider name, or attribute mapping values in GCP, you
      must also update the Entity ID, ACS URL, <br />
      or attribute mapping fields in the SAML IdP service provider spec,
      respectively.
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
      createSP={createSP}
      prevStep={prevStep}
      nextStep={nextStep}
      updateAgentMeta={updateAgentMeta}
      agentMeta={agentMeta}
      SpMetadataConfigComponent={AddMetadataGeneric}
      resourceSpec={resourceSpec}
    />
  );
}
