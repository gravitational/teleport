import React from 'react';
import { MemoryRouter } from 'react-router';

import { AgentMeta } from 'teleport/Discover/useDiscover';

import {
  AddServiceProvider as AddServiceProviderComponent,
  Props,
} from './AddServiceProvider';

export default {
  title: 'TeleportE/Discover/SAML Application/AddServiceProvider',
};

export const Default = () => {
  return (
    <MemoryRouter>
      <AddServiceProviderComponent {...props} attempt={{ status: '' }} />
    </MemoryRouter>
  );
};

export const Processing = () => {
  return (
    <MemoryRouter>
      <AddServiceProviderComponent
        {...props}
        attempt={{ status: 'processing' }}
      />
    </MemoryRouter>
  );
};

export const Failed = () => {
  return (
    <MemoryRouter>
      <AddServiceProviderComponent
        {...props}
        attempt={{
          status: 'failed',
          statusText: 'application already exists',
        }}
      />
    </MemoryRouter>
  );
};

const props: Props = {
  attempt: { status: '' },
  agentMeta: {
    resourceName: 'SAML Application',
    agentMatcherLabels: [],
  } as AgentMeta,
  updateAgentMeta: () => null,
  onSubmit: () => null,
  nextStep: () => null,
  prevStep: () => null,
};
