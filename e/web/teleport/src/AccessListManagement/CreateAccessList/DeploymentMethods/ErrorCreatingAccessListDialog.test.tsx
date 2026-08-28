import { render, screen } from 'design/utils/testing';

import { ErrorCreatingAccessListDialog } from './ErrorCreatingAccessListDialog';

test('renders acl cleanup command when access list cleanup is required', () => {
  render(
    <ErrorCreatingAccessListDialog
      error=""
      onCancel={jest.fn()}
      retry={jest.fn()}
      requiresCleanup={{
        accessListId: 'new-acl-id',
        error: 'failed to delete access list',
        roles: ['role-a'],
      }}
    />
  );

  expect(screen.getByText('failed to delete access list')).toBeInTheDocument();
  expect(screen.getByText('tctl acl rm new-acl-id')).toBeInTheDocument();
  expect(screen.getByText(/tctl rm role\/role-a/)).toBeInTheDocument();
});

test('renders role cleanup command when only role cleanup is required', () => {
  render(
    <ErrorCreatingAccessListDialog
      error=""
      onCancel={jest.fn()}
      retry={jest.fn()}
      requiresCleanup={{
        error: 'failed to delete access list',
        roles: ['role-a', 'role-b'],
      }}
    />
  );

  expect(screen.getByText('failed to delete access list')).toBeInTheDocument();
  expect(screen.queryByText(/tctl acl rm/)).not.toBeInTheDocument();
  expect(screen.getByText(/tctl rm role\/role-a/)).toBeInTheDocument();
  expect(screen.getByText(/tctl rm role\/role-b/)).toBeInTheDocument();
});

test('renders original error even if cleanup error exists', () => {
  render(
    <ErrorCreatingAccessListDialog
      error="the original error"
      onCancel={jest.fn()}
      retry={jest.fn()}
      requiresCleanup={{
        error: 'cleanup error',
        accessListId: 'some-id',
      }}
    />
  );

  expect(screen.getByText('the original error')).toBeInTheDocument();
  expect(screen.queryByText('cleanup error')).not.toBeInTheDocument();
  expect(
    screen.getByText(/Could not remove leftover resources/)
  ).toBeInTheDocument();
  expect(screen.getByText(/tctl acl rm some-id/)).toBeInTheDocument();
});
