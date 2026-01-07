import type { Plugin } from 'teleport/services/integrations';

export const entraPlugin: Plugin = {
  resourceType: 'plugin',
  kind: 'entra-id',
  name: 'entra-id-default',
  statusCode: 2,
  spec: {
    defaultOwners: ['emaster'],
    ssoConnectorId: 'entra-id',
    credentialSource: 'ENTRAID_CREDENTIALS_SOURCE_SYSTEM_CREDENTIALS',
    tenantId: '71cbeb2a-1b5b-44bd-909f-511a904a25b0',
    entraAppId: '9a5b7068-bb3c-4f76-9183-c0fbb6483b15',
    groupFilters: {
      id: [
        'd7055898-f95a-4432-9a95-574826c31543',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
      ],
      nameRegex: ['admin*', 'devops-prod*'],
      excludeId: [
        'a80b5881-d483-4724-8298-7a36017d6f94',
        'a80b5881-d483-4724-8298-7a36017d6f94',
      ],
      excludeNameRegex: ['finance*', 'sales*'],
    },
    accessGraphEnabled: true,
  },
  status: {
    code: 1,
    lastRun: new Date('2025-12-11T10:50:47.910628Z'),
    errorMessage: '',
    details: {
      imported_users: 296,
      imported_groups: 702,
    },
  },
};

export const entraPluginErrorStatus = {
  code: 2,
  lastRun: new Date('2025-12-11T10:50:47.910628Z'),
  errorMessage: 'Entra ID directory sync failed',
  lastRawError: `username "user@example.com" contains unsupported character(s), it should only include alphabets, hyphens, dots, and plus signs\n\tfailed to convert Entra ID user\nfailed to convert Entra ID group to Teleport access list. Error: expected Entra ID group(id=25f9c527-2314-414c-a75d-ef7efabcc99b) to have a non-empty display name\nfailed to convert Entra ID group to Teleport access list. Error: expected Entra ID group(id=080b50c3-1c98-4d8e-a54e-20143dbd4f99) to have a non-empty display name\nfailed to convert Entra ID group member 0c213fc5-7570-417b-affc-f4dbae5e002c\nfailed to convert Entra ID group member 0c213fc5-7570-417b-affc-f4dbae5e002c\n\tfailed to convert Entra ID group`,
  details: {
    imported_users: 296,
    imported_groups: 12,
  },
};
