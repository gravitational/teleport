import { StoryObj } from '@storybook/react-vite';
import { http, HttpResponse } from 'msw';
import { generatePath } from 'react-router';

import { Alert } from 'design/Alert';

import cfg from 'e-teleport/config';
import { allAccessAcl, fullAccess, noAccess } from 'teleport/mocks/contexts';
import { defaultRoleVersion } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { Role } from 'teleport/services/resources';
import { makeAcl } from 'teleport/services/user/makeAcl';

import { internalAccessListPresetLabelKey } from '../GuideEditor/Preset/TestHelper/roles';
import { rawAccessList } from './fixtures';
import { Provider } from './TestHelper/Provider';
import { ViewEditAccessList } from './ViewEditAccessList';

const presetAccessListId = 'preset-access-list-id';
const presetStandardRoleName = `access-standard-acl-preset-${presetAccessListId}`;
const presetAwsIcRoleName = `access-awsic-acl-preset-${presetAccessListId}`;

const accessListWithPreset = {
  ...rawAccessList,
  metadata: {
    ...rawAccessList.metadata,
    name: presetAccessListId,
    labels: {
      [internalAccessListPresetLabelKey]: 'long-term',
    },
  },
  spec: {
    ...rawAccessList.spec,
    title: 'Preset Access List',
    grants: {
      ...rawAccessList.spec.grants,
      roles: [presetStandardRoleName, presetAwsIcRoleName],
    },
  },
};

// Access list with only standard role (for InvalidRole story)
const accessListWithStandardRoleOnly = {
  ...accessListWithPreset,
  spec: {
    ...accessListWithPreset.spec,
    grants: {
      ...accessListWithPreset.spec.grants,
      roles: [presetStandardRoleName],
    },
  },
};

const presetStandardRole = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: {
    name: presetStandardRoleName,
    labels: {
      [internalAccessListPresetLabelKey]: presetAccessListId,
    },
  },
  spec: {
    allow: {
      // servers
      node_labels: { env: ['prod', 'staging'] },
      logins: ['root', 'ubuntu'],
      // apps
      app_labels: { team: ['engineering', 'platform'] },
      // dbs
      db_labels: { type: ['postgres', 'mysql'] },
      db_names: ['main_db', 'analytics_db'],
      db_users: ['admin', 'readonly'],
      // kube
      kubernetes_labels: { cluster: ['us-east', 'us-west'] },
      kubernetes_users: ['developer', 'deployer'],
      kubernetes_groups: ['system:masters', 'developers'],
      kubernetes_resources: [
        { kind: 'pod', name: '*', namespace: 'default' },
        { kind: 'deployment', name: 'web-*', namespace: 'production' },
      ],
      // windows desktops
      windows_desktop_labels: { department: ['finance', 'hr'] },
      windows_desktop_logins: ['Administrator', 'User'],
    },
    deny: {},
    options: {},
  },
};

const presetAwsIcRole = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: {
    name: presetAwsIcRoleName,
    labels: {
      [internalAccessListPresetLabelKey]: presetAccessListId,
    },
  },
  spec: {
    allow: {
      app_labels: { 'teleport.dev/origin': 'aws-identity-center' },
      account_assignments: [
        { account: '123456789012', permission_set: 'AdministratorAccess' },
        { account: '123456789012', permission_set: 'ReadOnlyAccess' },
        { account: '987654321098', permission_set: 'PowerUserAccess' },
      ],
    },
    deny: {},
    options: {},
  },
};

const accessListWithUnknownRoles = {
  ...accessListWithPreset,
  spec: {
    ...accessListWithPreset.spec,
    grants: {
      ...accessListWithPreset.spec.grants,
      roles: [presetStandardRoleName, 'unknown-role-not-matching-pattern'],
    },
  },
};

const commonHandlers = [
  http.get(cfg.oss.getUsersUrl(), () => {
    return HttpResponse.json([{ name: 'apple' }, { name: 'banana' }]);
  }),
  http.get(cfg.oss.getRoleUrl({ action: 'list' }), () => {
    return HttpResponse.json([{ name: 'admin' }, { name: 'access' }]);
  }),
  http.get(cfg.oss.api.unifiedResourcesPath.split('?')[0], () => {
    return HttpResponse.json({ items: [] });
  }),
];

function makeAccessListHandlers(accessList: typeof accessListWithPreset) {
  return [
    http.get(cfg.getAccessManagementListUrl(accessList.metadata.name), () => {
      return HttpResponse.json({ accessList });
    }),
    http.get(
      cfg.getAccessListUrl({
        action: 'reviews',
        params: { accessListId: accessList.metadata.name },
      }),
      () => {
        return HttpResponse.json({});
      }
    ),
    http.get(cfg.getAccessManagementListUrl(), () => {
      return HttpResponse.json({ accessLists: [accessList] });
    }),
  ];
}

function makeRolesHandler(roles: object[]) {
  return http.get('/v2/webapi/roles', () => {
    return HttpResponse.json({
      items: roles.map(role => ({ object: role })),
      startKey: '',
    });
  });
}

const accessListWithNoAccess = {
  ...accessListWithPreset,
  spec: {
    ...accessListWithPreset.spec,
    grants: {
      ...accessListWithPreset.spec.grants,
      roles: [], // no access
    },
  },
};

const presetStandardRoleWithDeny: Role = {
  kind: 'role',
  version: defaultRoleVersion,
  metadata: {
    name: presetStandardRoleName,
    labels: {
      [internalAccessListPresetLabelKey]: presetAccessListId,
    },
  },
  spec: {
    allow: {
      node_labels: { env: ['prod', 'staging'] },
      logins: ['root', 'ubuntu'],
    },
    deny: {
      node_labels: { env: ['restricted'] },
    },
    options: {} as any,
  },
};

export default {
  title: 'TeleportE/AccessLists/View/Preset',
};

export const WithAccess: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        ...makeAccessListHandlers(accessListWithPreset),
        makeRolesHandler([presetStandardRole, presetAwsIcRole]),
        ...commonHandlers,
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: accessListWithPreset.metadata.name,
          }),
        ]}
      >
        <DevNote />
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const NoReadPerm: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        ...makeAccessListHandlers(accessListWithPreset),
        makeRolesHandler([presetStandardRole]),
      ],
    },
  },
  render() {
    const restrictedAcl = makeAcl({
      ...allAccessAcl,
      accessList: fullAccess,
      roles: noAccess,
    });

    return (
      <Provider
        customAcl={restrictedAcl}
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: accessListWithPreset.metadata.name,
          }),
        ]}
      >
        <DevNote />
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const NoWritePerm: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        ...makeAccessListHandlers(accessListWithStandardRoleOnly),
        makeRolesHandler([presetStandardRoleWithDeny]),
        ...commonHandlers,
      ],
    },
  },
  render() {
    const restrictedAcl = makeAcl({
      ...allAccessAcl,
      accessList: fullAccess,
      roles: {
        ...fullAccess,
        create: false,
        edit: false,
        remove: false,
      },
    });

    return (
      <Provider
        customAcl={restrictedAcl}
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: accessListWithStandardRoleOnly.metadata.name,
          }),
        ]}
      >
        <DevNote />
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const InvalidRole: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        ...makeAccessListHandlers(accessListWithUnknownRoles),
        makeRolesHandler([presetStandardRoleWithDeny]),
        ...commonHandlers,
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: accessListWithStandardRoleOnly.metadata.name,
          }),
        ]}
      >
        <DevNote />
        <ViewEditAccessList />
      </Provider>
    );
  },
};

export const NoAccessDefined: StoryObj = {
  parameters: {
    msw: {
      handlers: [
        ...makeAccessListHandlers(accessListWithNoAccess),
        makeRolesHandler([]),
        ...commonHandlers,
      ],
    },
  },
  render() {
    return (
      <Provider
        initialEntries={[
          generatePath(cfg.routes.accessLists, {
            accessListId: accessListWithNoAccess.metadata.name,
          }),
        ]}
      >
        <DevNote />
        <ViewEditAccessList />
      </Provider>
    );
  },
};

function DevNote() {
  return (
    <Alert kind="neutral">Devs: go to &quot;Access Definition&quot; tab</Alert>
  );
}
