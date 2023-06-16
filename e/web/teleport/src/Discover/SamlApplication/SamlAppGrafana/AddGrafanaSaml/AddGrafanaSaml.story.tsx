import React from 'react';
import { MemoryRouter } from 'react-router';

import { AgentMeta } from 'teleport/Discover/useDiscover';

import {
  AddGrafanaSaml as AddGrafanaSamlComponent,
  Props,
} from './AddGrafanaSaml';

export default {
  title: 'TeleportE/Discover/SAML Application/AddGrafanaSaml',
};

export const Default = () => {
  return (
    <MemoryRouter>
      <AddGrafanaSamlComponent {...props} attempt={{ status: '' }} />
    </MemoryRouter>
  );
};

export const Processing = () => {
  return (
    <MemoryRouter>
      <AddGrafanaSamlComponent {...props} attempt={{ status: 'processing' }} />
    </MemoryRouter>
  );
};

export const Failed = () => {
  return (
    <MemoryRouter>
      <AddGrafanaSamlComponent
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
    resourceName: 'SAML Application (Grafana)',
    agentMatcherLabels: [],
  } as AgentMeta,
  updateAgentMeta: () => null,
  onSubmit: () => null,
  nextStep: () => null,
  prevStep: () => null,
};
