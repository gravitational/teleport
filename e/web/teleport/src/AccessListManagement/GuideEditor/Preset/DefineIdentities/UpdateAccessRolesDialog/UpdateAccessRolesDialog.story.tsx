import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { delay, http, HttpResponse } from 'msw';
import { MemoryRouter } from 'react-router';

import cfg from 'teleport/config';

import {
  AccessListManagementContext,
  AccessListManagementContextValue,
} from '../../../../AccessListManagementContext';
import { GuideEditorState } from '../../../useGuideEditor';
import { AccessRoleEditor } from '../../../ViewAndEditAccessRoles/types';
import { makeHandlers } from '../../TestHelper/mocks';
import { UpdateAccessRolesDialog as Component } from './UpdateAccessRolesDialog';

export default {
  title: 'TeleportE/AccessLists/Guide/UpdateAccessListDialog',
  parameters: {
    msw: {
      handlers: [
        ...makeHandlers(),
        http.delete(cfg.api.role.delete, async () => {
          await delay(300);
          return HttpResponse.json({});
        }),
      ],
    },
  },
};

export function SuccessDefault() {
  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: async () => [],
    onClose: () => {},
  };

  return (
    <Wrapper>
      <Component
        accessRoleEditor={accessRoleEditor}
        onCancelUpdate={() => {}}
      />
    </Wrapper>
  );
}

export function SuccessWithRolesToDelete() {
  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: async () => [
      { name: 'access-standard-acl-preset-old-role' },
      { name: 'access-awsic-acl-preset-deprecated-role' },
    ],
    onClose: () => {},
  };

  return (
    <Wrapper>
      <Component
        accessRoleEditor={accessRoleEditor}
        onCancelUpdate={() => {}}
      />
    </Wrapper>
  );
}

export function Loading() {
  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: () => new Promise(() => {}), // simulates loading
    onClose: () => {},
  };

  return (
    <Wrapper>
      <Component
        accessRoleEditor={accessRoleEditor}
        onCancelUpdate={() => {}}
      />
    </Wrapper>
  );
}

export function Failed() {
  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: async () => {
      throw new Error('Whoops some kind of error');
    },
    onClose: () => {},
  };

  return (
    <Wrapper>
      <Component
        accessRoleEditor={accessRoleEditor}
        onCancelUpdate={() => {}}
      />
    </Wrapper>
  );
}

const mockGuideEditor: GuideEditorState = {
  preset: 'long-term',
  setPreset: () => {},
  onGuideSelect: () => {},
  views: [],
  emitEvent: () => {},
  currentStep: 0,
  setCurrentStep: () => {},
  prevStep: () => {},
  nextStep: () => {},
  reset: () => {},
  awsIcRoleState: {
    roleConditions: { account_assignments: [] },
    setRoleConditions: () => {},
    roleEditState: { original: null, isDirty: false },
    initRoleEditState: () => {},
    getRoleToSave: () => null,
    addAccount: () => {},
    removeAccount: () => {},
    addAccountWildcard: () => {},
    removeAccountWildcard: () => {},
    updateAccount: () => {},
  } as any,
  standardRoleState: {
    roleConditions: {},
    setRoleConditions: () => {},
    roleEditState: { original: null, isDirty: false },
    initRoleEditState: () => {},
    getRoleToSave: () => null,
    appIdentityFields: { requiredFields: [], allPagesFetched: true },
    markAppIdentityFieldsAsRequired: () => {},
  } as any,
  definedAccess: () => false,
  definedAccessInAnyRoleCondition: () => false,
  undoEditRoleChanges: () => {},
  isEditing: false,
  getRolesToSave: () => [],
  removeLocationState: () => null,
  getResumableState: () => null,
  originatedFromOkta: false,
};

const mockContextValue: AccessListManagementContextValue = {
  guideEditor: mockGuideEditor,
} as AccessListManagementContextValue;

const queryClient = new QueryClient();

const Wrapper = ({ children }: { children: React.ReactNode }) => (
  <MemoryRouter>
    <QueryClientProvider client={queryClient}>
      <AccessListManagementContext.Provider value={mockContextValue}>
        {children}
      </AccessListManagementContext.Provider>
    </QueryClientProvider>
  </MemoryRouter>
);
