import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { ContextProvider } from 'teleport';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';

import cfg from 'e-teleport/config';

import { CreateAccessList } from './CreateAccessList';

const { worker, rest } = window.msw;

const defaultIsTeamFlag = cfg.oss.isTeam;
const defaultIsEnterprise = cfg.oss.isEnterprise;
const defaultIsIgsEnabled = cfg.oss.isIgsEnabled;
const defaultCreateLimit = cfg.oss.featureLimits.accessListCreateLimit;
const defaultIsCloud = cfg.oss.isCloud;

export default {
  title: 'Teleport/AccessLists/Create',
  decorators: [
    Story => {
      cfg.oss.isEnterprise = true;
      // Reset request handlers added in individual stories.
      worker.resetHandlers();
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isTeam = defaultIsTeamFlag;
          cfg.oss.isEnterprise = defaultIsEnterprise;
          cfg.oss.isIgsEnabled = defaultIsIgsEnabled;
          cfg.oss.featureLimits.accessListCreateLimit = defaultCreateLimit;
          cfg.oss.isCloud = defaultIsCloud;
        };
      }, []);
      return <Story />;
    },
  ],
};

export const Failed = () => {
  worker.use(
    rest.get(cfg.oss.api.usersPath, (req, res, ctx) => {
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
    rest.get(cfg.oss.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.status(200));
    }),
    rest.get(cfg.oss.api.rolesPath, (req, res, ctx) => {
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
    rest.get(cfg.oss.getRolesUrl(), (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.oss.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.getAccessManagementListUrl(), (req, res, ctx) => {
      return res.once(ctx.json({ accessLists: [] }));
    })
  );
  return (
    <Provider>
      <CreateAccessList />
    </Provider>
  );
};

export const LoadedWithIgs = () => {
  cfg.oss.isIgsEnabled = true;
  cfg.oss.isCloud = true;
  worker.use(
    rest.get(cfg.oss.getRolesUrl(), (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.oss.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.getAccessManagementListUrl(), (req, res, ctx) => {
      return res.once(ctx.json({ accessLists: [] }));
    })
  );
  return (
    <Provider>
      <CreateAccessList />
    </Provider>
  );
};

export const LoadedReachedLimit = () => {
  cfg.oss.featureLimits.accessListCreateLimit = 1;
  cfg.oss.isCloud = true;
  worker.use(
    rest.get(cfg.oss.getRolesUrl(), (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.oss.api.usersPath, (req, res, ctx) => {
      return res.once(ctx.json([]));
    }),
    rest.get(cfg.getAccessManagementListUrl(), (req, res, ctx) => {
      return res.once(
        ctx.json({
          accessLists: [
            {
              metadata: { name: 'aaa' },
              spec: {
                title: 'Interns',
                description: 'lorem ipsum description',
                audit: { frequency: '', next_audit_date: new Date() },
                grants: { roles: ['access', 'editor'] },
                ownership_requires: { roles: [] },
                owners: [],
              },
              membersCount: 0,
            },
          ],
        })
      );
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
