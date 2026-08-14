import { Alert, Box, ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import { OutlineWarn } from 'design/Alert/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { UserDisplayName } from 'shared/components/UserDisplayName';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  AccessList,
  AccessListOrigin,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';

import { AccessListModified } from './Shared';

type Base = {
  username: string;
  listTitle?: string;
  displayPrimary?: string;
  onClose(): void;
  accessList: AccessListModified;
  updateAccessList(accessList: AccessList): void;
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
  listTitle,
  displayPrimary,
  onClose,
  accessList,
  updateAccessList,
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
      .then(resp => {
        onClose();
        updateAccessList(resp);
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
        <P1>
          Are you sure you want to delete {kind}{' '}
          {listTitle ? (
            <Text as="span" bold color="text.main">
              {listTitle}
            </Text>
          ) : (
            <UserDisplayName
              username={username}
              primaryText={displayPrimary}
              primaryTextProps={{ fontWeight: 'bold' }}
              layout="inline"
            />
          )}{' '}
          ?
        </P1>
        {accessList.origin === AccessListOrigin.Okta && kind === 'Member' && (
          <Box mt={4}>
            <DeleteMemberWarning />
          </Box>
        )}
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

export const DeleteMemberWarning = ({
  isReviewing = false,
}: {
  isReviewing?: boolean;
}) => (
  <OutlineWarn mb={2}>
    {isReviewing
      ? 'Changes made here will be reflected in Okta. '
      : 'This change will be reflected in Okta. '}
    {isReviewing ? 'Members removed' : 'This user'} will also be unassigned from
    the targeted Okta group or application.
  </OutlineWarn>
);
