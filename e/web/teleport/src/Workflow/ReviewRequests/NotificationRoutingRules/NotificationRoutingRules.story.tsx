import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { rest } from 'msw';

import { ContextProvider } from 'teleport';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';

import cfg from 'e-teleport/config';

import { NotificationRoutingRulesDialog } from './NotificationRoutingRulesDialog';

initialize();
const defaultIsCloud = cfg.oss.isCloud;

export default {
  title: 'TeleportE/Workflow/NotificationRoutingRules',
  loaders: [mswLoader],
  decorators: [
    Story => {
      useEffect(() => {
        // Clean up
        return () => {
          cfg.oss.isCloud = defaultIsCloud;
        };
      }, []);
      return <Story />;
    },
  ],
};

const validRuleObject = {
  metadata: { name: 'valid-default-to-standard-editor' },
  spec: {
    subjects: ['access_request'],
    condition: 'contains_any(access_request.spec.roles, set("access"))',
    notification: {
      name: 'plugin-name',
      recipients: ['apple', 'banana', 'carrot'],
    },
  },
};

const invalidRuleObject = {
  metadata: { name: 'invalid-fields-default-to-yaml-editor' },
  spec: {
    subjects: ['access_request'],
    condition: 'invalid field',
    notification: {
      name: 'plugin-name',
      recipients: ['apple', 'banana', 'carrot'],
    },
  },
};

const ruleYaml = `kind: access_monitoring_rule
metadata:
name: sdfssd
spec:
condition: some-condition
notification:
name: mattermost
recipients:
- apple
- banana
- carrot
subjects:
- access_request
version: v1`;

const withPlugins = rest.get(cfg.getPluginUrl(), (req, res, ctx) =>
  res(
    ctx.json([
      {
        name: 'slack-plugin1',
        details: '',
        statusCode: '',
        type: 'slack',
        spec: {},
      },
      {
        name: 'slack-plugin2',
        details: '',
        statusCode: '',
        type: 'slack',
        spec: {},
      },
    ])
  )
);

const withRule = rest.get(cfg.api.accessMonitoringRule.list, (req, res, ctx) =>
  res(
    ctx.json({
      rules: [
        {
          object: {
            ...validRuleObject,
            metadata: { name: 'valid-default-to-standard-editor' },
          },
          yaml: ``,
        },
      ],
      startKey: '',
    })
  )
);

const noRules = rest.get(cfg.api.accessMonitoringRule.list, (req, res, ctx) =>
  res(ctx.json({ rules: [], startKey: '' }))
);

const deleteRule = rest.delete(
  cfg.api.accessMonitoringRule.delete,
  (req, res, ctx) => res(ctx.json({}))
);

const createRule = rest.post(
  cfg.api.accessMonitoringRule.create,
  async (req, res, ctx) => {
    const json = await req.json();
    return res(
      ctx.json({
        object: json.object,
        yaml: '',
      })
    );
  }
);

export const WithValidRule = () => {
  return <Component />;
};
WithValidRule.parameters = {
  msw: {
    handlers: [
      withRule,
      withPlugins,
      deleteRule,
      createRule,
      rest.post(cfg.oss.api.yaml.parse, (req, res, ctx) =>
        res(ctx.json({ resource: validRuleObject }))
      ),
      rest.post(cfg.oss.api.yaml.stringify, (req, res, ctx) =>
        res(ctx.json({ yaml: ruleYaml }))
      ),
    ],
  },
};

export const WithAnInvalidRule = () => {
  return <Component />;
};
WithAnInvalidRule.parameters = {
  msw: {
    handlers: [
      rest.get(cfg.api.accessMonitoringRule.list, (req, res, ctx) =>
        res(
          ctx.json({
            rules: [
              {
                object: {
                  ...invalidRuleObject,
                  metadata: { name: 'invalid-default-to-yaml-editor' },
                },
                yaml: ruleYaml,
              },
            ],
            startKey: '',
          })
        )
      ),
      withPlugins,
      deleteRule,
      createRule,
      rest.post(cfg.oss.api.yaml.parse, (req, res, ctx) =>
        res(ctx.json({ resource: invalidRuleObject }))
      ),
      rest.post(cfg.oss.api.yaml.stringify, (req, res, ctx) =>
        res(ctx.json({ yaml: ruleYaml }))
      ),
    ],
  },
};

// Click on "create" button on story
// to see the no plugin state.
export const NoPlugins = () => {
  return <Component />;
};
NoPlugins.parameters = {
  msw: {
    handlers: [
      noRules,
      rest.get(cfg.getPluginUrl(), (req, res, ctx) => res(ctx.json([]))),
      deleteRule,
      createRule,
    ],
  },
};

export const WithListRuleErrors = () => {
  return <Component />;
};
WithListRuleErrors.parameters = {
  msw: {
    handlers: [
      withPlugins,
      deleteRule,
      createRule,
      rest.get(cfg.api.accessMonitoringRule.list, (req, res, ctx) =>
        res(ctx.status(404), ctx.json({ message: 'some listing rules error' }))
      ),
    ],
  },
};

export const WithPluginError = () => {
  return <Component />;
};
WithPluginError.parameters = {
  msw: {
    handlers: [
      withRule,
      deleteRule,
      createRule,
      rest.get(cfg.getPluginUrl(), (req, res, ctx) =>
        res(ctx.status(404), ctx.json({ message: 'some listing plugin error' }))
      ),
    ],
  },
};

export const WithDeleteError = () => {
  return <Component />;
};
WithDeleteError.parameters = {
  msw: {
    handlers: [
      withRule,
      createRule,
      withPlugins,
      rest.delete(cfg.api.accessMonitoringRule.delete, (req, res, ctx) =>
        res(ctx.status(404), ctx.json({ message: 'some delete error' }))
      ),
    ],
  },
};

export const WithCreateError = () => {
  return <Component />;
};
WithCreateError.parameters = {
  msw: {
    handlers: [
      withRule,
      withPlugins,
      deleteRule,
      rest.post(cfg.api.accessMonitoringRule.create, async (req, res, ctx) => {
        return res(ctx.status(404), ctx.json({ message: 'some create error' }));
      }),
    ],
  },
};

export const WithNoPerm = () => {
  return <Component noAccess={true} />;
};
WithNoPerm.parameters = {
  msw: {
    handlers: [
      withRule,
      withPlugins,
      deleteRule,
      createRule,
      rest.post(cfg.oss.api.yaml.parse, (req, res, ctx) =>
        res(ctx.json({ resource: validRuleObject }))
      ),
      rest.post(cfg.oss.api.yaml.stringify, (req, res, ctx) =>
        res(ctx.json({ yaml: ruleYaml }))
      ),
    ],
  },
};

const Component = ({ noAccess = false }: { noAccess?: boolean }) => {
  const ctx = createTeleportContext();
  ctx.storeUser.state.acl = getAcl({ noAccess: noAccess });
  return (
    <MemoryRouter initialEntries={[{ pathname: '' }]}>
      <ContextProvider ctx={ctx}>
        <NotificationRoutingRulesDialog
          onClose={() => null}
          transitionState="entered"
        />
      </ContextProvider>
    </MemoryRouter>
  );
};
