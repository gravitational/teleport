import { delay, http, HttpHandler, HttpResponse } from 'msw';

import { UnifiedResourceApp } from 'shared/components/UnifiedResources';
import { getErrorMessage } from 'shared/utils/error';

import cfg from 'e-teleport/config';
import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { gitServers } from 'teleport/GitServers/fixtures';
import { kubes } from 'teleport/Kubes/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';
import { ResourceAccessKind } from 'teleport/Roles/RoleEditor/StandardEditor/standardmodel';
import { PermissionSet } from 'teleport/services/apps';
import makeApp from 'teleport/services/apps/makeApps';

import { DefinableResourceAccessFields } from '../role/listaccess';

// The path won't match if fetch is made with query params defined,
// so its removed to match regardless of query params.
export const unifiedResourcePath =
  cfg.oss.api.unifiedResourcesPath.split('?')[0];

const accessListPathWithoutQuery =
  cfg.api.accessListManagementPathV2.split('?')[0];

export function makeHandlers(handlers: HttpHandler[] = []) {
  return [
    http.post(cfg.api.accessListPreset.terraform, () => new HttpResponse()),
    http.post(cfg.oss.api.captureUserEventPath, () => new HttpResponse()),
    http.get(accessListPathWithoutQuery, () => {
      return HttpResponse.json({ accessLists: [] });
    }),
    http.get(cfg.getPluginUrl('okta', 'get'), () => {
      return HttpResponse.json({});
    }),
    http.get(cfg.oss.api.userPreferencesPath, () => {
      return HttpResponse.json([{}]);
    }),
    http.get('/v1/webapi/roles/:name', ({ params }) => {
      return HttpResponse.json({
        kind: 'role',
        id: params.name,
        name: params.name,
        content: `kind: role
metadata:
  name: ${params.name}
version: v7
spec:
  allow: {}
  deny: {}
  options: {}`,
      });
    }),
    http.post('/v1/webapi/yaml/parse/:kind', () => {
      return HttpResponse.json({
        resource: {
          kind: 'role',
          version: 'v7',
          metadata: { name: 'role-name' },
          spec: {
            allow: {},
            deny: {},
            options: {
              forward_agent: false,
              max_session_ttl: '30h0m0s',
              port_forwarding: true,
              cert_format: 'standard',
              client_idle_timeout: '0s',
              disconnect_expired_cert: false,
              enhanced_recording: ['command', 'network'],
              bpf: ['command', 'network'],
              desktop_clipboard: true,
              desktop_directory_sharing: true,
              create_host_user: false,
              create_host_user_mode: 'off',
              create_db_user: false,
              create_db_user_mode: 'off',
              idp: { saml: { enabled: true } },
              create_desktop_user: false,
              ssh_file_copy: true,
            },
          },
        },
      });
    }),
    ...handlers,
  ];
}

export function fetchUnifiedResources(
  action: 'loading' | 'any-error' | 'get',
  items: any[] = []
) {
  if (action === 'loading') {
    return http.get(unifiedResourcePath, () => delay('infinite'));
  }
  if (action === 'any-error') {
    return http.get(unifiedResourcePath, () => {
      return HttpResponse.json(
        {
          error: { message: 'Whoops, listing unified resources error' },
        },
        { status: 500 }
      );
    });
  }
  if (action === 'get') {
    if (items === null) {
      return http.get(unifiedResourcePath, h => {
        const params = new URL(h.request.url).searchParams;
        const resourceKind = params.get('kinds') as ResourceAccessKind;

        let items = [];
        switch (resourceKind) {
          case 'app':
            items = [
              makeApp({
                name: 'AppTestRow',
                uri: '',
                publicAddr: '',
                description: '',
                awsConsole: false,
                labels: [
                  { name: 'env', value: 'test' },
                  { name: 'test', value: 'apple' },
                ],
                clusterId: '',
                fqdn: '',
              }),
              makeApp({
                name: 'AppTestRow2',
                uri: '',
                publicAddr: '',
                description: '',
                awsConsole: false,
                labels: [
                  { name: 'env', value: 'test2' },
                  { name: 'test2', value: 'banana' },
                ],
                clusterId: '',
                fqdn: '',
              }),
              makeApp({
                name: 'AppTestRow3',
                uri: '',
                publicAddr: '',
                description: '',
                awsConsole: false,
                labels: [{ name: 'env', value: 'test3' }],
                clusterId: '',
                fqdn: '',
              }),
              ...apps,
            ];
            break;
          case 'db':
            items = [
              {
                kind: 'db',
                name: 'DbTestRow',
                description: '',
                type: '',
                protocol: 'postgres',
                labels: [{ name: 'env', value: 'test' }],
                hostname: '',
              },
              ...databases,
            ];
            break;
          case 'git_server':
            items = [
              {
                kind: 'git_server',
                id: 'abc',
                clusterId: '',
                hostname: 'github',
                subKind: 'github',
                labels: [],
                github: {
                  organization: 'GitServerTestRow',
                  integration: 'GitServerTestRow',
                },
              },
              ...gitServers,
            ];
            break;
          case 'kube_cluster':
            items = [
              {
                kind: 'kube_cluster',
                name: 'KubeClusterTestRow',
                labels: [{ name: 'env', value: 'test' }],
              },
              ...kubes,
            ];
            break;
          case 'node':
            items = [
              {
                kind: 'node',
                subKind: 'teleport',
                tunnel: false,
                sshLogins: [],
                id: 'NodeTestRow',
                clusterId: '',
                hostname: 'NodeTestRow',
                addr: '172.10.1.20:3022',
                tags: [{ name: 'env', value: 'test' }],
              },
              ...nodes.map(n => ({ ...n, tags: n.labels })),
            ];
            break;
          case 'windows_desktop':
            items = [
              {
                kind: 'windows_desktop',
                os: 'windows',
                name: 'WindowsTestRow',
                addr: 'host.com',
                labels: [{ name: 'env', value: 'test' }],
                logins: [],
              },
              ...desktops,
            ];
            break;
          case 'linux_desktop':
            items = [
              {
                kind: 'linux_desktop',
                name: 'LinuxTestRow',
                addr: 'host.com',
                labels: [{ name: 'env', value: 'test' }],
                logins: [],
              },
              ...desktops,
            ];
            break;
          default:
            resourceKind satisfies never;
        }

        return HttpResponse.json({
          items,
        });
      });
    }
    return http.get(unifiedResourcePath, () => {
      return HttpResponse.json({
        items,
      });
    });
  }
}

export const appsWithoutPermissionSets = [
  { kind: 'app', name: 'app-name-1', friendlyName: 'app-friendly-name-1' },
  { kind: 'app', name: 'app-name-2', friendlyName: 'app-friendly-name-2' },
  { kind: 'app', name: 'app-name-3', friendlyName: 'app-friendly-name-3' },
  { kind: 'app', name: 'app-name-4', friendlyName: 'app-friendly-name-4' },
  { kind: 'app', name: 'app-name-5', friendlyName: 'app-friendly-name-5' },
  { kind: 'app', name: 'app-name-6', friendlyName: 'app-friendly-name-6' },
  { kind: 'app', name: 'app-name-7', friendlyName: 'app-friendly-name-7' },
  { kind: 'app', name: 'app-name-8', friendlyName: 'app-friendly-name-8' },
  { kind: 'app', name: 'app-name-9', friendlyName: 'app-friendly-name-9' },
  { kind: 'app', name: 'app-name-10', friendlyName: 'app-friendly-name-10' },
  { kind: 'app', name: 'app-name-11', friendlyName: 'app-friendly-name-11' },
  { kind: 'app', name: 'app-name-12', friendlyName: 'app-friendly-name-12' },
  { kind: 'app', name: 'app-name-13', friendlyName: 'app-friendly-name-13' },
  { kind: 'app', name: 'app-name-14', friendlyName: 'app-friendly-name-14' },
  { kind: 'app', name: 'app-name-15', friendlyName: 'app-friendly-name-15' },
  { kind: 'app', name: 'app-name-16', friendlyName: 'app-friendly-name-16' },
  { kind: 'app', name: 'app-name-17', friendlyName: 'app-friendly-name-17' },
  { kind: 'app', name: 'app-name-18', friendlyName: 'app-friendly-name-18' },
  { kind: 'app', name: 'app-name-19', friendlyName: 'app-friendly-name-19' },
];

export const appsWithSomePermissionSets = [
  {
    kind: 'app',
    name: 'app-name-1 (has permission set)',
    friendlyName: 'app-friendly-name-1',
    permissionSets: [{ name: 'ps-name-1', arn: 'ps-arn-1' }],
  },
  {
    kind: 'app',
    name: 'app-name-2 (has permissions set)',
    friendlyName: 'app-friendly-name-2',
    permissionSets: [
      { name: 'ps-name-1', arn: 'ps-arn-1' },
      { name: 'ps-name-2', arn: 'ps-arn-2' },
    ],
  },
  {
    kind: 'app',
    name: 'app-name-3  (has permissions set)',
    friendlyName: 'app-friendly-name-3',
    permissionSets: [
      { name: 'ps-name-1', arn: 'ps-arn-1' },
      { name: 'ps-name-2', arn: 'ps-arn-2' },
      { name: 'ps-name-3', arn: 'ps-arn-3' },
    ],
  },
  {
    kind: 'app',
    name: 'app-name-4 (has permissions set)',
    friendlyName: 'app-friendly-name-4',
    permissionSets: [
      { name: 'ps-name-1', arn: 'ps-arn-1' },
      { name: 'ps-name-2', arn: 'ps-arn-2' },
      { name: 'ps-name-3', arn: 'ps-arn-3' },
    ],
  },
  {
    kind: 'app',
    name: 'app-name-5 (has permissions set)',
    friendlyName: 'app-friendly-name-5',
    permissionSets: [{ name: 'ps-name-4', arn: 'ps-arn-4' }],
  },
  { kind: 'app', name: 'app-name-6', friendlyName: 'app-friendly-name-6' },
];

export const appsWithAllMatchingPermissionSet = [
  {
    kind: 'app',
    name: 'app-name-1 (has permission set)',
    friendlyName: 'app-friendly-name-1',
    permissionSets: [
      { name: 'ps-name-1', arn: 'ps-arn-1' },
      { name: 'ps-name-2', arn: 'ps-arn-2' },
    ],
  },
  {
    kind: 'app',
    name: 'app-name-2 (has permissions set)',
    friendlyName: 'app-friendly-name-2',
    permissionSets: [
      { name: 'ps-name-1', arn: 'ps-arn-1' },
      { name: 'ps-name-2', arn: 'ps-arn-2' },
    ],
  },
  {
    kind: 'app',
    name: 'app-name-3  (has permissions set)',
    friendlyName: 'app-friendly-name-3',
    permissionSets: [
      { name: 'ps-name-1', arn: 'ps-arn-1' },
      { name: 'ps-name-2', arn: 'ps-arn-2' },
      { name: 'ps-name-3', arn: 'ps-arn-3' },
    ],
  },
];

export const sampleSelectedPermissionSet: PermissionSet[] = [
  {
    name: 'ps-name-1',
    arn: 'ps-arn-1',
    assignmentId: 'ps-assignment-id',
  },
  {
    name: 'ps-name-2',
    arn: 'ps-arn-2',
    assignmentId: 'ps-assignment-id',
  },
];

export const sampleSelectedAccounts: UnifiedResourceApp[] = [
  {
    kind: 'app',
    labels: [],
    description: '',
    id: 'fd166661-2917-50dd-badc-4f0c92f241a2',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-1',
    friendlyName: 'app-friendly-name-1',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '8041929f-b15e-590f-8b31-5d952d887b6b',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-2',
    friendlyName: 'app-friendly-name-2',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '0a2c2dc9-2241-5b80-9800-a5d42a100d00',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-3',
    friendlyName: 'app-friendly-name-3',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: 'ebd4764a-965f-5f25-ac0f-2430221c5468',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-4',
    friendlyName: 'app-friendly-name-4',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '1f7cd185-c127-5ff4-b1d8-23dc7c5572e7',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-5',
    friendlyName: 'app-friendly-name-5',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: 'e3907952-9577-5f3a-a47e-e7d2f0204795',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-6',
    friendlyName: 'app-friendly-name-6',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: 'fa397f74-1379-522e-a4a7-d9f456adf5e4',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-7',
    friendlyName: 'app-friendly-name-7',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '56a64193-d404-5560-8371-2682b0079766',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-8',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '358f5616-40c6-5918-a31e-65044a148982',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-9',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '7bf12a12-970a-59bf-9df1-baf315de8df6',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-10',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '235bd1d5-c802-5655-a706-a46ee2ba158b',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-11',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '1a13f98d-4ccc-5502-af8a-9b73ea1b58cb',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-12',
    friendlyName: 'app-friendly-name-12',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: 'b70a6d20-5689-50ac-9308-ac114e62911c',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-13',
    friendlyName: 'app-friendly-name-13',
  },
  {
    kind: 'app',
    labels: [],
    description: '',
    id: '257640ce-3bab-52e2-8b5a-9bd5e8a085de',
    awsConsole: false,
    samlApp: false,
    name: 'app-name-14',
    friendlyName: 'app-friendly-name-14',
  },
];

export async function withCustomError(
  field: DefinableResourceAccessFields,
  processCallback: () => Promise<any>
) {
  try {
    await processCallback();
  } catch (error) {
    // Provide custom error message.
    throw new Error(`For field "${field}": ${getErrorMessage(error)}`, {
      cause: error,
    });
  }
}
