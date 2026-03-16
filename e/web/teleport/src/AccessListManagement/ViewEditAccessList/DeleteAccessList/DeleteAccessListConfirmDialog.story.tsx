import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { addWeeks } from 'date-fns';
import { delay, http, HttpResponse } from 'msw';

import { Info } from 'design/Alert';

import cfgE from 'e-teleport/config';
import {
  AccessListMemberKind,
  AccessListOrigin,
  AccessListType,
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';
import cfg from 'teleport/config';

import { Provider } from '../../GuideEditor/Preset/TestHelper/Provider';
import type { AccessListModified } from '../Shared';
import { DeleteAccessListConfirmDialog } from './DeleteAccessListConfirmDialog';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
    },
  },
});

function StoryProvider({ children }: { children: React.ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <Provider>{children}</Provider>
    </QueryClientProvider>
  );
}

export default {
  title: 'TeleportE/AccessLists/DeleteAccessList',
  beforeEach: () => {
    queryClient.clear();
  },
};

export const Default = () => {
  return (
    <StoryProvider>
      <DeleteAccessListConfirmDialog
        onClose={() => null}
        accessList={baseAccessList}
      />
    </StoryProvider>
  );
};

export const WithOktaOrigin = () => {
  return (
    <StoryProvider>
      <Info mb={3}>
        Shows additional warning about Okta sync when deleting an Okta-origin
        access list
      </Info>
      <DeleteAccessListConfirmDialog
        onClose={() => null}
        accessList={mockAccessListOkta}
      />
    </StoryProvider>
  );
};

export const WithPresetLoading = () => {
  return (
    <StoryProvider>
      <DeleteAccessListConfirmDialog
        onClose={() => null}
        accessList={mockAccessListWithPreset}
      />
    </StoryProvider>
  );
};
WithPresetLoading.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getRoleUrl({ action: 'listv2' }), () => delay('infinite')),
    ],
  },
};

export const WithPresetFetchError = () => {
  return (
    <StoryProvider>
      <DeleteAccessListConfirmDialog
        onClose={() => null}
        accessList={mockAccessListWithPreset}
      />
    </StoryProvider>
  );
};
WithPresetFetchError.parameters = {
  msw: {
    handlers: [
      http.get(cfg.getRoleUrl({ action: 'listv2' }), async () => {
        return HttpResponse.json(
          { message: 'Failed to fetch roles: whoops error' },
          { status: 500 }
        );
      }),
    ],
  },
};

export const WithPresetRolesToBeDeletedTable = () => {
  return (
    <StoryProvider>
      <Info mb={3}>Click delete to see the list of roles to be deleted</Info>
      <DeleteAccessListConfirmDialog
        onClose={() => null}
        accessList={mockAccessListWithPreset}
      />
    </StoryProvider>
  );
};
WithPresetRolesToBeDeletedTable.parameters = {
  msw: {
    handlers: [
      http.delete(
        cfgE.getAccessManagementListUrl(':accessListId'),
        async () => {
          return HttpResponse.json({});
        }
      ),
      http.get(cfg.getRoleUrl({ action: 'listv2' }), async () => {
        return HttpResponse.json({
          items: [
            {
              id: 'id1',
              kind: 'role',
              name: 'preset-role-abc123',
              content: '',
              object: {
                metadata: {
                  labels: {
                    'teleport.internal/access-list-preset':
                      'b59c9b50-b534-52ca-870e-9f7069b205dc',
                  },
                },
              },
            },
            {
              id: 'id2',
              kind: 'role',
              name: 'preset-role-def456',
              content: '',
              object: {
                metadata: {
                  labels: {
                    'teleport.internal/access-list-preset':
                      'b59c9b50-b534-52ca-870e-9f7069b205dc',
                  },
                },
              },
            },
          ],
        });
      }),
    ],
  },
};

const baseAccessList: AccessListModified = {
  id: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
  type: AccessListType.Default,
  title: 'Engineering Team',
  metadata: {
    name: 'b59c9b50-b534-52ca-870e-9f7069b205dc',
    labels: {},
    revision: '',
  },
  audit: {
    recurrence: {
      frequency: ReviewFrequency.OneYear,
      dayOfMonth: ReviewDayOfMonth.FifteenthDayOfMonth,
    },
    nextDate: addWeeks(new Date(), 4),
  },
  grants: {
    roles: ['developer', 'viewer'],
    traits: { team: ['engineering'] },
    traitLabels: [],
    traitList: [],
    scopedRoles: [],
  },
  ownerGrants: {
    roles: ['admin'],
    traits: {},
    traitLabels: [],
    traitList: [],
    scopedRoles: [],
  },
  ownershipRequires: {
    roles: ['admin'],
    traits: {},
    traitLabels: [],
    traitList: [],
  },
  membershipRequires: {
    roles: ['access'],
    traits: {},
    traitLabels: [],
    traitList: [],
  },
  owners: [
    {
      name: 'admin-user',
      title: 'Admin',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  members: [
    {
      name: 'developer1',
      title: 'Developer 1',
      joined: new Date(),
      addedBy: 'admin-user',
      membershipKind: AccessListMemberKind.User,
    },
  ],
  requiresReview: false,
  inheritedMemberGrants: { roles: [], traits: {}, scopedRoles: [] },
};

const mockAccessListOkta: AccessListModified = {
  ...baseAccessList,
  id: 'okta-access-list-id',
  title: 'Okta Synced Group',
  origin: AccessListOrigin.Okta,
};

const mockAccessListWithPreset: AccessListModified = {
  ...baseAccessList,
  title: 'Long Term Access',
  preset: 'long-term',
};
