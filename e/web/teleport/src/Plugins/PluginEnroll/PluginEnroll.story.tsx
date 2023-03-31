import React from 'react';
import { MemoryRouter } from 'react-router';

import { PluginEnroll } from './PluginEnroll';

export default {
  title: 'TeleportE/PluginEnroll',
};

export const Processing = () => (
  <MemoryRouter>
    <PluginEnroll {...sample} attempt={{ status: 'processing' as any }} />
  </MemoryRouter>
);

export const NoPluginsEnrolled = () => (
  <MemoryRouter>
    <PluginEnroll {...sample} />
  </MemoryRouter>
);

export const SlackAlreadyEnrolled = () => (
  <MemoryRouter>
    <PluginEnroll {...sample} existingTypes={['slack']} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <PluginEnroll
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
};
