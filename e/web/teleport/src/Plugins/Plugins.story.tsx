import React from 'react';

import { Plugins } from './Plugins';

export default {
  title: 'TeleportE/Plugins',
};

export function Processing() {
  return <Plugins {...sample} attempt={{ status: 'processing' as any }} />;
}

export function Loaded() {
  return <Plugins {...sample} />;
}

export function Empty() {
  return <Plugins {...sample} items={[]} />;
}

export function Failed() {
  return (
    <Plugins
      {...sample}
      attempt={{ status: 'failed', statusText: 'some error message' }}
    />
  );
}

const plugins = [
  {
    name: 'slack-default',
    type: 'slack',
    niceType: 'Slack',
    status: {
      code: 'Running',
    },
    details: `Messages will be sent to assigned reviewers and the "#access-requests" channel`,
  },

  {
    name: 'slack-secondary',
    type: 'slack',
    niceType: 'Slack',
    status: {
      code: 'Unknown',
    },
    details: `Messages will be sent to assigned reviewers and the "#access-requests" channel`,
  },

  {
    name: 'acmeco-default',
    type: 'acmeco', // unknown plugin, should handle gracefuly
    niceType: 'AcmeCo.',
    status: {
      code: 'Unauthorized',
    },
    details: `Messages will be sent to assigned reviewers and the "#access-requests" channel`,
  },
];

const sample = {
  attempt: {
    status: 'success',
  } as const,
  operation: { type: 'none' } as const,
  items: plugins,
  onCancelDelete: () => {},
  onDelete: () => Promise.resolve(),
  onStartDelete: () => {},
  run: () => Promise.resolve(true),
  setOperation: () => {},
};
