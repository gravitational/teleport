import React, { useEffect } from 'react';
import { MemoryRouter } from 'react-router';
import { http, HttpResponse } from 'msw';
import { withoutQuery } from 'web/packages/build/storybook';

import { ContextProvider } from 'teleport';
import { createTeleportContext, getAcl } from 'teleport/mocks/contexts';

import cfg from 'e-teleport/config';

import { NotificationRoutingRulesDialog } from './NotificationRoutingRulesDialog';

const defaultIsCloud = cfg.oss.isCloud;

const accessMonitoringRuleListWithoutQuery = withoutQuery(
  cfg.api.accessMonitoringRule.list
);

export default {
  title: 'TeleportE/AccessRequests/NotificationRoutingRules',
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
      name: 'slack-plugin',
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

const withPlugins = http.get(cfg.getPluginUrl(), () =>
  HttpResponse.json([
    {
      name: 'slack-plugin',
      details: '',
      statusCode: '',
      type: 'slack',
      spec: { fallbackChannel: 'some-fallback-channel' },
    },
    {
      name: 'mattermost-plugin',
      details: '',
      statusCode: '',
      type: 'mattermost',
      spec: { channel: 'some-channel', reportToEmail: 'foo@example.com' },
    },
    {
      name: 'mattermost-plugin-with-only-channel',
      details: '',
      statusCode: '',
      type: 'mattermost',
      spec: { channel: 'some-channel' },
    },
    {
      name: 'mattermost-plugin-with-only-email',
      details: '',
      statusCode: '',
      type: 'mattermost',
      spec: { reportToEmail: 'foo@example.com' },
    },
    {
      name: 'opgsgenie',
      details: '',
      statusCode: '',
      type: 'opsgenie',
      spec: { defaultSchedules: ['schedule1', 'schedule2', 'schedule3'] },
    },
  ])
);

const withRule = http.get(accessMonitoringRuleListWithoutQuery, () =>
  HttpResponse.json({
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
);

const noRules = http.get(accessMonitoringRuleListWithoutQuery, () =>
  HttpResponse.json({ rules: [], startKey: '' })
);

const deleteRule = http.delete(cfg.api.accessMonitoringRule.delete, () =>
  HttpResponse.json({})
);

const createRule = http.post(
  cfg.api.accessMonitoringRule.create,
  async ({ request }) => {
    const json = (await request.json()) as { object: string };
    return HttpResponse.json({
      object: json.object,
      yaml: '',
    });
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
      http.post(cfg.oss.api.yaml.parse, () =>
        HttpResponse.json({
          resource: validRuleObject,
        })
      ),
      http.post(cfg.oss.api.yaml.stringify, () =>
        HttpResponse.json({
          aml: ruleYaml,
        })
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
      http.get(accessMonitoringRuleListWithoutQuery, () =>
        HttpResponse.json({
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
      ),
      withPlugins,
      deleteRule,
      createRule,
      http.post(cfg.oss.api.yaml.parse, () =>
        HttpResponse.json({ resource: invalidRuleObject })
      ),
      http.post(cfg.oss.api.yaml.stringify, () =>
        HttpResponse.json({ yaml: ruleYaml })
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
      http.get(cfg.getPluginUrl(), () => HttpResponse.json([])),
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
      http.get(accessMonitoringRuleListWithoutQuery, () =>
        HttpResponse.json(
          {
            message: 'some listing rules error',
          },
          { status: 404 }
        )
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
      http.get(cfg.getPluginUrl(), () =>
        HttpResponse.json(
          {
            message: 'some listing plugin error',
          },
          { status: 404 }
        )
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
      http.delete(cfg.api.accessMonitoringRule.delete, () =>
        HttpResponse.json(
          {
            message: 'some delete error',
          },
          { status: 404 }
        )
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
      http.post(cfg.api.accessMonitoringRule.create, () => {
        HttpResponse.json(
          {
            message: 'some create error',
          },
          {
            status: 404,
          }
        );
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
      http.post(cfg.oss.api.yaml.parse, () =>
        HttpResponse.json({ resource: validRuleObject })
      ),
      http.post(cfg.oss.api.yaml.stringify, () =>
        HttpResponse.json({ yaml: ruleYaml })
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
