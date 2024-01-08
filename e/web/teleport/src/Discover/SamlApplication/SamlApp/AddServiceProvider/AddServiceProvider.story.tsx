import React from 'react';
import { MemoryRouter } from 'react-router';

import { AgentMeta } from 'teleport/Discover/useDiscover';

import { ServiceProvider, SPProps } from './AddServiceProvider';

export default {
  title: 'TeleportE/Discover/SAML Application/AddServiceProvider',
};

export const Default = () => {
  return (
    <MemoryRouter>
      <ServiceProvider {...props} attempt={{ status: '' }} />
    </MemoryRouter>
  );
};

export const Processing = () => {
  return (
    <MemoryRouter>
      <ServiceProvider {...props} attempt={{ status: 'processing' }} />
    </MemoryRouter>
  );
};

export const Failed = () => {
  return (
    <MemoryRouter>
      <ServiceProvider
        {...props}
        attempt={{
          status: 'failed',
          statusText: 'application already exists',
        }}
      />
    </MemoryRouter>
  );
};

const props: SPProps = {
  header: 'Add Service Provider To Teleport',
  subtitle:
    "Please refer to your Service Provider's documentation for instruction's on how to obtain the Entity ID and ACS URL.",
  attempt: { status: '' },
  agentMeta: {
    resourceName: 'SAML Application',
    agentMatcherLabels: [],
  } as AgentMeta,
  updateAgentMeta: () => null,
  createSP: () => null,
  nextStep: () => null,
  prevStep: () => null,
};
