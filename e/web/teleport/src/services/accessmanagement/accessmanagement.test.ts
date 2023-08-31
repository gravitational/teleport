import api from 'teleport/services/api';

import { accessManagementService } from './accessmanagement';

test('fetch access lists, empty responses does not throw error', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ accessLists: null });
  let response = await accessManagementService.fetchAccessLists();
  expect(response).toStrictEqual([]);

  jest.spyOn(api, 'get').mockResolvedValue({ accessLists: [{}] });

  response = await accessManagementService.fetchAccessLists();
  expect(response).toStrictEqual([
    {
      id: '',
      title: '',
      description: '',
      owners: [],
      members: [],
      grants: {
        roles: [],
        traits: {},
      },
      audit: {
        frequency: '',
        nextDate: undefined,
      },
      ownershipRequires: {
        roles: [],
        traits: {},
      },
      membershipRequires: {
        roles: [],
        traits: {},
      },
    },
  ]);
});

test('fetch an access list, empty response does not throw error', async () => {
  jest.spyOn(api, 'get').mockResolvedValue({ accessList: null });
  let response = await accessManagementService.fetchAccessList(
    'does-not-matter'
  );
  expect(response).toStrictEqual({
    audit: { frequency: '', nextDate: undefined },
    description: '',
    grants: { roles: [], traits: {} },
    id: '',
    members: [],
    membershipRequires: { roles: [], traits: {} },
    owners: [],
    ownershipRequires: { roles: [], traits: {} },
    title: '',
  });
});
