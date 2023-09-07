import React from 'react';
import { ButtonSecondary, ButtonWarning, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';

import { AccessListModified } from './ViewEditAccessList';

type Base = {
  username: string;
  onClose(): void;
  accessList: AccessListModified;
  fetchAccessList(): Promise<void | boolean>;
};

type PropForMember = Base & {
  kind: 'Member';
};
type PropForOwner = Base & {
  kind: 'Owner';
};

export function DeleteUserConfirmDialog({
  kind,
  username,
  onClose,
  accessList,
  fetchAccessList,
}: PropForMember | PropForOwner) {
  const { members: existingMembers, owners: existingOwners } = accessList;
  const { attempt, setAttempt } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function handleOnDelete() {
    setAttempt({ status: 'processing' });
    let req: Partial<AccessList>;

    switch (kind) {
      case 'Member':
        const updatedMembers = existingMembers.filter(u => u.name !== username);
        req = { members: updatedMembers };
        break;

      case 'Owner':
        const updatedOwners = existingOwners.filter(u => u.name !== username);
        req = { owners: updatedOwners };
    }

    accessManagementService
      .updateAccessList({ req, original: accessList })
      .then(() => {
        onClose();
        fetchAccessList();
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete {kind}?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Text typography="paragraph" mb="6">
          Are you sure you want to delete {kind}{' '}
          <Text as="span" bold color="text.main">
            {username}
          </Text>{' '}
          ?
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={handleOnDelete}>
          Yes, Delete {kind}
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
