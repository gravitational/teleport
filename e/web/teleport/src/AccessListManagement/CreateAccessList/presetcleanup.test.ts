import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { ApiError } from 'teleport/services/api/parseError';
import ResourceService from 'teleport/services/resources';

import {
  cleanupPresetAcl,
  PendingCleanup,
  tryPresetAclCleanup,
} from './presetcleanup';

const mfaResponse = {} as any;

jest.mock('e-teleport/services/accessmanagement', () => ({
  accessManagementService: {
    fetchAccessList: jest.fn(),
    deleteAccessListWithPreset: jest.fn(),
  },
}));

describe('cleanupPresetAccessList', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('records pending verification when acl is not immediately found', async () => {
    const originalError = new Error('internal failure');
    const updateRequiresCleanup = jest.fn();

    jest
      .mocked(accessManagementService.fetchAccessList)
      .mockRejectedValue(makeApiError(404, 'not found'));

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    expect(accessManagementService.fetchAccessList).toHaveBeenCalledWith(
      'new-acl-id'
    );
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).not.toHaveBeenCalled();
    expect(updateRequiresCleanup).toHaveBeenCalledWith({
      verifyAccessListExists: true,
    });
  });

  test('does nothing when acl already exists with initial create', async () => {
    const originalError = makeApiError(
      409,
      'access list new-acl-id already exists'
    );
    const updateRequiresCleanup = jest.fn();

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    expect(accessManagementService.fetchAccessList).not.toHaveBeenCalled();
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).not.toHaveBeenCalled();
    expect(updateRequiresCleanup).not.toHaveBeenCalled();
  });

  test('records pending cleanup when fetching existing acl (to verify it exists before deleting) fails with non 404', async () => {
    const originalError = new Error('some non 404 err');
    const updateRequiresCleanup = jest.fn();

    jest
      .mocked(accessManagementService.fetchAccessList)
      .mockRejectedValue(makeApiError(500, 'backend failing'));

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    expect(accessManagementService.fetchAccessList).toHaveBeenCalledWith(
      'new-acl-id'
    );
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).not.toHaveBeenCalled();
    expect(updateRequiresCleanup).toHaveBeenCalledWith({
      error: 'backend failing',
      accessListId: 'new-acl-id',
    });
  });

  test('verified acl exists, but acl deletion fails', async () => {
    const originalError = new Error('some non 404 err');
    const updateRequiresCleanup = jest.fn();

    jest.mocked(accessManagementService.fetchAccessList).mockResolvedValue({
      id: 'new-acl-id',
      metadata: {
        labels: {
          'teleport.internal/access-list-preset-roles': 'role-a,role-b',
        },
      },
    } as any);
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockRejectedValue(makeApiError(500, 'backend failing'));

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    expect(accessManagementService.fetchAccessList).toHaveBeenCalledWith(
      'new-acl-id'
    );
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).toHaveBeenCalledWith('new-acl-id', mfaResponse);
    expect(updateRequiresCleanup).toHaveBeenCalledWith({
      accessListId: 'new-acl-id',
      roles: ['role-a', 'role-b'],
      error: 'backend failing',
    });
  });

  test('cleanup succeeds (all resources successfully deleted)', async () => {
    const originalError = new Error('some non 404 err');
    const updateRequiresCleanup = jest.fn();

    jest.mocked(accessManagementService.fetchAccessList).mockResolvedValue({
      id: 'new-acl-id',
      metadata: {
        labels: {
          'teleport.internal/access-list-preset-roles': 'role-a,role-b',
        },
      },
    } as any);

    // Tests that the roles returned by successful delete overwrites
    // what was extracted from the acl labels above.
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockResolvedValue(['got-role-a', 'got-role-b']);
    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockResolvedValue(undefined);

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    expect(accessManagementService.fetchAccessList).toHaveBeenCalledWith(
      'new-acl-id'
    );
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).toHaveBeenCalledWith('new-acl-id', mfaResponse);
    expect(updateRequiresCleanup).not.toHaveBeenCalled();

    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledTimes(2);
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-a',
      mfaResponse
    );
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-b',
      mfaResponse
    );
  });

  test('records pending cleanup when some role deletion fail', async () => {
    const originalError = new Error('some non 404 err');
    const updateRequiresCleanup = jest.fn();

    jest.mocked(accessManagementService.fetchAccessList).mockResolvedValue({
      id: 'new-acl-id',
      metadata: { labels: {} },
    } as any);
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockResolvedValue(['got-role-a', 'got-role-b', 'got-role-c']);

    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockImplementation((role: string) => {
        if (role === 'got-role-b') {
          return Promise.reject(
            makeApiError(500, 'failed deleting got-role-b')
          );
        }
        return Promise.resolve();
      });

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    // Tests despite a role failure, remaining deletion of roles continue to delete.
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledTimes(3);
    expect(updateRequiresCleanup).toHaveBeenCalledWith({
      roles: ['got-role-b'],
      error: 'failed deleting got-role-b',
    });
  });

  test('404 role deletion error is ignored and counts as "success"', async () => {
    const originalError = new Error('some non 404 err');
    const updateRequiresCleanup = jest.fn();

    jest.mocked(accessManagementService.fetchAccessList).mockResolvedValue({
      id: 'new-acl-id',
      metadata: { labels: {} },
    } as any);
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockResolvedValue(['got-role-a', 'got-role-b', 'got-role-c']);

    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockImplementation((role: string) => {
        if (role === 'got-role-b') {
          return Promise.reject(makeApiError(404, 'not found'));
        }
        return Promise.resolve();
      });

    await expect(
      cleanupPresetAcl(
        'new-acl-id',
        originalError,
        mfaResponse,
        updateRequiresCleanup
      )
    ).rejects.toThrow(originalError);

    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledTimes(3);
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-a',
      mfaResponse
    );
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-b',
      mfaResponse
    );
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-c',
      mfaResponse
    );
    expect(updateRequiresCleanup).not.toHaveBeenCalled();
  });
});

describe('retryCleanupPresetAcl', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  test('retry with pending verification clears cleanup when acl still does not exist', async () => {
    jest
      .mocked(accessManagementService.fetchAccessList)
      .mockRejectedValue(makeApiError(404, 'not found'));

    const pendingCleanup: PendingCleanup = {
      verifyAccessListExists: true,
    };

    await expect(
      tryPresetAclCleanup('new-acl-id', pendingCleanup, mfaResponse)
    ).resolves.toBeNull();

    expect(accessManagementService.fetchAccessList).toHaveBeenCalledWith(
      'new-acl-id'
    );
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).not.toHaveBeenCalled();
    expect(ResourceService.prototype.deleteRole).not.toHaveBeenCalled();
  });

  test('retry with pending verification performs cleanup when access list is found', async () => {
    jest.mocked(accessManagementService.fetchAccessList).mockResolvedValue({
      id: 'new-acl-id',
      metadata: {
        labels: {
          'teleport.internal/access-list-preset-roles': 'role-a',
        },
      },
    } as any);
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockResolvedValue(['role-a']);
    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockResolvedValue(undefined);

    const pendingCleanup: PendingCleanup = {
      verifyAccessListExists: true,
    };

    await expect(
      tryPresetAclCleanup('new-acl-id', pendingCleanup, mfaResponse)
    ).resolves.toBeNull();

    expect(accessManagementService.fetchAccessList).toHaveBeenCalledWith(
      'new-acl-id'
    );
    expect(
      accessManagementService.deleteAccessListWithPreset
    ).toHaveBeenCalledWith('new-acl-id', mfaResponse);
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'role-a',
      mfaResponse
    );
  });

  test('retry with both accessListId and roles, deletes both', async () => {
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockResolvedValue(['got-role-a', 'got-role-b']);

    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockResolvedValue(undefined);

    const pendingCleanup: PendingCleanup = {
      accessListId: 'new-acl-id',
      // Will get replaced by actual roles that need deleting
      // when delete is successful.
      roles: ['saved-role-a', 'saved-role-b'],
      error: 'some prev cleanup error',
    };

    await expect(
      tryPresetAclCleanup('new-acl-id', pendingCleanup, mfaResponse)
    ).resolves.toBeNull();

    expect(
      accessManagementService.deleteAccessListWithPreset
    ).toHaveBeenCalledWith('new-acl-id', mfaResponse);

    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledTimes(2);
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-a',
      mfaResponse
    );
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-b',
      mfaResponse
    );
    expect(accessManagementService.fetchAccessList).not.toHaveBeenCalled();
  });

  test('retry with only roles, should only delete roles', async () => {
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockResolvedValue(['got-role-a']);

    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockResolvedValue(undefined);

    const pendingCleanup: PendingCleanup = {
      roles: ['got-role-a'],
      error: 'some prev cleanup error',
    };

    await expect(
      tryPresetAclCleanup('new-acl-id', pendingCleanup, mfaResponse)
    ).resolves.toBeNull();

    expect(
      accessManagementService.deleteAccessListWithPreset
    ).not.toHaveBeenCalled();
    expect(accessManagementService.fetchAccessList).not.toHaveBeenCalled();

    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledTimes(1);
    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledWith(
      'got-role-a',
      mfaResponse
    );
  });

  test('records pending cleanup when retrying to delete access list fails', async () => {
    jest
      .mocked(accessManagementService.deleteAccessListWithPreset)
      .mockRejectedValue(makeApiError(500, 'backend failing'));

    const pendingCleanup: PendingCleanup = {
      accessListId: 'new-acl-id',
      error: 'some prev cleanup error',
    };

    await expect(
      tryPresetAclCleanup('new-acl-id', pendingCleanup, mfaResponse)
    ).resolves.toStrictEqual({
      accessListId: 'new-acl-id',
      error: 'backend failing',
      roles: [],
    });

    expect(
      accessManagementService.deleteAccessListWithPreset
    ).toHaveBeenCalledWith('new-acl-id', mfaResponse);

    expect(ResourceService.prototype.deleteRole).not.toHaveBeenCalled();
    expect(accessManagementService.fetchAccessList).not.toHaveBeenCalled();
  });

  test('records pending cleanup when retrying to delete roles fail', async () => {
    jest
      .spyOn(ResourceService.prototype, 'deleteRole')
      .mockRejectedValue(makeApiError(500, 'backend failing'));

    const pendingCleanup: PendingCleanup = {
      roles: ['saved-role-a', 'saved-role-b'],
      error: 'some prev cleanup error',
    };

    await expect(
      tryPresetAclCleanup('new-acl-id', pendingCleanup, mfaResponse)
    ).resolves.toStrictEqual({
      roles: ['saved-role-a', 'saved-role-b'],
      error: 'backend failing',
    });

    expect(
      accessManagementService.deleteAccessListWithPreset
    ).not.toHaveBeenCalled();

    expect(ResourceService.prototype.deleteRole).toHaveBeenCalledTimes(2);
    expect(accessManagementService.fetchAccessList).not.toHaveBeenCalled();
  });
});

function makeApiError(status: number, message = 'error') {
  return new ApiError({
    message,
    response: new Response(null, { status }),
  });
}
