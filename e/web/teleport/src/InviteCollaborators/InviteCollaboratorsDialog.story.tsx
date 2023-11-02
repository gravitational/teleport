import React from 'react';
import { MemoryRouter } from 'react-router';

import { ContextProvider } from 'teleport';
import { createTeleportContext } from 'teleport/mocks/contexts';

import cfg from 'e-teleport/config';

import { InviteCollaboratorsDialog } from './InviteCollaboratorsDialog';

export default {
  title: 'TeleportE/InviteCollaborators',
  decorators: [
    Story => {
      // Reset request handlers added in individual stories.
      worker.resetHandlers();
      return <Story />;
    },
  ],
};

const { worker, rest } = window.msw;

export const Dialog = () => {
  worker.use(
    rest.get(cfg.oss.getUsersUrl(), (req, res, ctx) => {
      return res.once(
        ctx.json([
          { name: 'apple' },
          { name: 'banana' },
          {
            name: 'carrot',
            roles: ['reviewer', 'auditor'],
            allTraits: { fruit: ['carrot'] },
          },
        ])
      );
    }),
    rest.get(cfg.oss.getRolesUrl(), (req, res, ctx) => {
      return res.once(
        ctx.json([
          { name: 'admin', description: 'admin' },
          { name: 'auditor', description: 'auditor' },
          { name: 'reviewer', description: 'reviewer' },
          { name: 'access', description: 'access' },
          { name: 'editor' },
        ])
      );
    })
  );

  const ctx = createTeleportContext() as any;
  ctx.cloudService = { sendTeleportInvite: () => Promise.resolve([]) };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InviteCollaboratorsDialog onClose={() => {}} open={true} />
      </ContextProvider>
    </MemoryRouter>
  );
};

export const DialogError = () => {
  worker.use(
    rest.get(cfg.oss.getUsersUrl(), (req, res, ctx) => {
      return res.once(
        ctx.status(500),
        ctx.json({ message: 'testing error for getUsers()' })
      );
    }),
    rest.get(cfg.oss.getRolesUrl(), (req, res, ctx) => {
      return res.once(
        ctx.status(500),
        ctx.json({ message: 'testing error for getRoles()' })
      );
    })
  );

  const ctx = createTeleportContext() as any;
  ctx.cloudService = { sendTeleportInvite: () => Promise.resolve([]) };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InviteCollaboratorsDialog onClose={() => {}} open={true} />
      </ContextProvider>
    </MemoryRouter>
  );
};

export const DialogSpinner = () => {
  const ctx = createTeleportContext() as any;
  ctx.cloudService = { sendTeleportInvite: () => Promise.resolve([]) };
  ctx.userService = { fetchUsers: () => new Promise(() => {}) };
  ctx.resourceService = { fetchRoles: () => new Promise(() => {}) };

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>
        <InviteCollaboratorsDialog onClose={() => {}} open={true} />
      </ContextProvider>
    </MemoryRouter>
  );
};
