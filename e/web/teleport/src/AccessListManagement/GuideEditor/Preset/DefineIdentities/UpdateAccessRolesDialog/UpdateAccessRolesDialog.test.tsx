import { QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { setupServer } from 'msw/node';
import { MemoryRouter } from 'react-router';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
  waitFor,
} from 'design/utils/testing';

import * as accessListManagementContext from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  makeHandlers,
  unifiedResourcePath,
} from 'e-teleport/AccessListManagement/GuideEditor/Preset/TestHelper/mocks';
import { GuideEditorState } from 'e-teleport/AccessListManagement/GuideEditor/useGuideEditor';
import ResourceService from 'teleport/services/resources';

import { AccessRoleEditor } from '../../../ViewAndEditAccessRoles/types';
import { UpdateAccessRolesDialog } from './UpdateAccessRolesDialog';

const server = setupServer(
  http.get(unifiedResourcePath, () => {
    return HttpResponse.json({ items: [] });
  }),
  ...makeHandlers()
);

beforeAll(() => {
  server.listen();
});

beforeEach(async () => {
  await testQueryClient.resetQueries();
});

afterEach(() => {
  jest.resetAllMocks();
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

function mockGuideEditor() {
  const reset = jest.fn();
  const getRolesToSave = jest.fn().mockReturnValue([]);

  jest
    .spyOn(accessListManagementContext, 'useAccessListManagementContext')
    .mockReturnValue({
      guideEditor: {
        reset,
        getRolesToSave,
        preset: 'long-term',
        setPreset: jest.fn(),
        currentStep: 0,
        setCurrentStep: jest.fn(),
        prevStep: jest.fn(),
        nextStep: jest.fn(),
        awsIcRoleState: undefined,
        standardRoleState: undefined,
        definedAccess: jest.fn().mockReturnValue(false),
        definedAccessInAnyRoleCondition: jest.fn().mockReturnValue(false),
        undoEditRoleChanges: jest.fn(),
        isEditing: false,
        getResumableState: jest.fn(),
        originatedFromOkta: false,
        removeLocationState: jest.fn(),
      } as GuideEditorState,
    } as any);

  return { reset, getRolesToSave };
}

test('rendering loading state upon initial render', async () => {
  mockGuideEditor();

  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: () => new Promise(() => {}), // Never resolves
    onClose: jest.fn(),
  };

  render(
    <Provider accessRoleEditor={accessRoleEditor} onCancelUpdate={jest.fn()} />
  );

  await waitFor(() => {
    expect(screen.getByTestId('indicator')).toBeInTheDocument();
  });

  expect(screen.getByRole('button', { name: /done/i })).toBeDisabled();
  expect(screen.getByRole('button', { name: /cancel/i })).toBeDisabled();
});

test('rendering error with retry button when update fails', async () => {
  const { reset } = mockGuideEditor();

  const onClose = jest.fn();
  const onUpdateAccess = jest
    .fn()
    .mockRejectedValue(new Error('Failed to update access roles'));

  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess,
    onClose,
  };

  render(
    <Provider accessRoleEditor={accessRoleEditor} onCancelUpdate={jest.fn()} />
  );

  await screen.findByText(/failed to update access roles/i);
  expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /cancel/i })).toBeInTheDocument();

  // Clicking retry should attempt to update again
  onUpdateAccess.mockResolvedValue([]);
  await userEvent.click(screen.getByRole('button', { name: /retry/i }));

  await screen.findByText(/successfully saved changes/i);

  // Cancel button should be gone
  expect(
    screen.queryByRole('button', { name: /cancel/i })
  ).not.toBeInTheDocument();

  // Finish the dialog
  await userEvent.click(screen.getByRole('button', { name: /done/i }));

  expect(reset).toHaveBeenCalled();
  expect(onClose).toHaveBeenCalled();
});

test('clicking on cancel, closes the dialog', async () => {
  mockGuideEditor();

  const onCancelUpdate = jest.fn();
  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: jest.fn().mockRejectedValue(new Error('Update failed')),
    onClose: jest.fn(),
  };

  render(
    <Provider
      accessRoleEditor={accessRoleEditor}
      onCancelUpdate={onCancelUpdate}
    />
  );

  await screen.findByText(/update failed/i);
  await userEvent.click(screen.getByRole('button', { name: /cancel/i }));

  expect(onCancelUpdate).toHaveBeenCalled();
});

test('rendering of table and deleting roles from table for a access list using preset', async () => {
  const { reset } = mockGuideEditor();

  const rolesToDelete = [
    { name: 'role-to-delete-1' },
    { name: 'role-to-delete-2' },
  ];

  const onClose = jest.fn();
  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: jest.fn().mockResolvedValue(rolesToDelete),
    onClose,
  };

  jest
    .spyOn(ResourceService.prototype, 'deleteRole')
    .mockResolvedValue(undefined);

  render(
    <Provider accessRoleEditor={accessRoleEditor} onCancelUpdate={jest.fn()} />
  );

  await screen.findByText(
    /successfully saved changes. optionally delete the roles below/i
  );
  expect(screen.getByText('role-to-delete-1')).toBeInTheDocument();
  expect(screen.getByText('role-to-delete-2')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: /close/i })).toBeInTheDocument();

  // Test deleting a role.
  const removeButtons = screen.getAllByRole('button', { name: /remove/i });
  await userEvent.click(removeButtons[0]);

  await waitFor(() => {
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'role-to-delete-1'
    );
  });

  await waitFor(() => {
    expect(screen.queryByText('role-to-delete-1')).not.toBeInTheDocument();
  });
  expect(screen.getByText('role-to-delete-2')).toBeInTheDocument();

  // Test deleting the last role.
  await userEvent.click(screen.getByRole('button', { name: /remove/i }));

  await waitFor(() => {
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'role-to-delete-2'
    );
  });

  await waitFor(() => {
    expect(screen.queryByText('role-to-delete-2')).not.toBeInTheDocument();
  });
  expect(screen.getByText(/all roles deleted/i)).toBeInTheDocument();

  // Finish and close the dialog
  await userEvent.click(screen.getByRole('button', { name: /close/i }));

  expect(reset).toHaveBeenCalled();
  expect(onClose).toHaveBeenCalled();
});

test('rendering error when deleting a role fails and successfuly retry', async () => {
  mockGuideEditor();

  const rolesToDelete = [
    { name: 'role-to-delete-1' },
    { name: 'role-to-delete-2' },
  ];

  const accessRoleEditor: AccessRoleEditor = {
    onUpdateAccess: jest.fn().mockResolvedValue(rolesToDelete),
    onClose: jest.fn(),
  };

  const deleteRoleSpy = jest
    .spyOn(ResourceService.prototype, 'deleteRole')
    .mockRejectedValue(new Error('Failed to delete role'));

  render(
    <Provider accessRoleEditor={accessRoleEditor} onCancelUpdate={jest.fn()} />
  );

  await screen.findByText(
    /successfully saved changes. optionally delete the roles below/i
  );

  // Trigger error
  await userEvent.click(screen.getAllByRole('button', { name: /remove/i })[0]);
  await screen.findByText(/failed to delete role/i);

  expect(screen.getByText('role-to-delete-1')).toBeInTheDocument();
  expect(screen.getByText('role-to-delete-2')).toBeInTheDocument();

  // Retry with success response
  deleteRoleSpy.mockResolvedValue(undefined);
  await userEvent.click(screen.getAllByRole('button', { name: /remove/i })[0]);

  await waitFor(() => {
    expect(screen.queryByText('role-to-delete-1')).not.toBeInTheDocument();
  });

  // Error should be cleared after successful deletion
  expect(screen.queryByText(/failed to delete role/i)).not.toBeInTheDocument();
});

const Provider = ({
  accessRoleEditor,
  onCancelUpdate,
}: {
  accessRoleEditor: AccessRoleEditor;
  onCancelUpdate: () => void;
}) => {
  return (
    <MemoryRouter>
      <QueryClientProvider client={testQueryClient}>
        <UpdateAccessRolesDialog
          accessRoleEditor={accessRoleEditor}
          onCancelUpdate={onCancelUpdate}
        />
      </QueryClientProvider>
    </MemoryRouter>
  );
};
