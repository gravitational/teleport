import React from 'react';
import { MemoryRouter } from 'react-router';

import { IntegrationEnroll } from './IntegrationEnroll';

export default {
  title: 'TeleportE/IntegrationEnroll',
};

export const Processing = () => (
  <MemoryRouter>
    <IntegrationEnroll {...sample} attempt={{ status: 'processing' as any }} />
  </MemoryRouter>
);

export const NoPluginsEnrolled = () => (
  <MemoryRouter>
    <IntegrationEnroll {...sample} />
  </MemoryRouter>
);

export const SlackAlreadyEnrolled = () => (
  <MemoryRouter>
    <IntegrationEnroll {...sample} existingTypes={['slack']} />
  </MemoryRouter>
);

export const NoAccessToPlugins = () => (
  <MemoryRouter>
    <IntegrationEnroll
      {...sample}
      existingTypes={['slack']}
      hasPluginAccess={false}
    />
  </MemoryRouter>
);

export const NoAccessToIntegrations = () => (
  <MemoryRouter>
    <IntegrationEnroll
      {...sample}
      existingTypes={['slack']}
      hasIntegrationAccess={false}
    />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <IntegrationEnroll
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const sample = {
  attempt: {
    status: 'success' as const,
  },
  availableTypes: ['slack'],
  existingTypes: [],
  run: () => Promise.resolve(true),
  hasPluginAccess: true,
  hasIntegrationAccess: true,
};
