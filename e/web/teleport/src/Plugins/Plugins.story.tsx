import React from 'react';
import { MemoryRouter } from 'react-router';

import { Plugins } from './Plugins';

import type { Plugin } from 'e-teleport/services/plugins';

export default {
  title: 'TeleportE/Plugins',
};

export const Processing = () => (
  <MemoryRouter>
    <Plugins {...sample} attempt={{ status: 'processing' as any }} />
  </MemoryRouter>
);

export const Loaded = () => (
  <MemoryRouter>
    <Plugins {...sample} />
  </MemoryRouter>
);

export const Empty = () => (
  <MemoryRouter>
    <Plugins {...sample} items={[]} />
  </MemoryRouter>
);

export const Failed = () => (
  <MemoryRouter>
    <Plugins
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  </MemoryRouter>
);

const plugins: Plugin[] = [
  {
    name: 'slack-default',
    type: 'slack',
    status: {
      code: 'Running',
    },
    details: `Messages will be sent to assigned reviewers and the "#access-requests" channel`,
  },

  {
    name: 'slack-another',
    type: 'slack',
    status: {
      code: 'Unknown',
    },
    details: `Messages will be sent to assigned reviewers and the "#access-requests" channel`,
  },

  {
    name: 'acmeco-default',
    type: 'acmeco', // unknown plugin type, should handle gracefuly
    status: {
      code: 'Unauthorized',
    },
    details: `Messages will be sent to assigned reviewers and the "#access-requests" channel`,
  },
];

const sample = {
  attempt: {
    status: 'success' as const,
  },
  operation: { type: 'none' } as const,
  items: plugins,
  onCancelDelete: () => {},
  onDelete: () => Promise.resolve(),
  onStartDelete: () => {},
  run: () => Promise.resolve(true),
  setOperation: () => {},
};
