import React from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';

import cfg from 'teleport/config';

import { CreateAccessList } from './CreateAccessList';

const { worker, rest } = window.msw;

export default {
  title: 'Teleport/AccessLists/Create',
  decorators: [
    Story => {
      // Reset request handlers added in individual stories.
      worker.resetHandlers();
      return <Story />;
    },
  ],
};

export const Failed = () => {
  worker.use(
    rest.get(cfg.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.status(500));
    })
  );
  return (
    <Provider>
      <CreateAccessList />
    </Provider>
  );
};

export const NoAccess = () => {
  worker.use(
    rest.get(cfg.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.status(200));
    }),
    rest.get(cfg.api.rolesPath, (req, res, ctx) => {
      return res.once(ctx.status(200));
    })
  );
  return (
    <Provider customAcl={getAcl({ noAccess: true })}>
      <CreateAccessList />
    </Provider>
  );
};

export const Loaded = () => {
  worker.use(
    rest.get(cfg.getRolesUrl(), (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.json([]));
    })
  );
  return (
    <Provider>
      <CreateAccessList />
    </Provider>
  );
};

const Provider = props => {
  const ctx = createTeleportContext({ customAcl: props.customAcl });

  return (
    <MemoryRouter>
      <ContextProvider ctx={ctx}>{props.children}</ContextProvider>
    </MemoryRouter>
  );
};
