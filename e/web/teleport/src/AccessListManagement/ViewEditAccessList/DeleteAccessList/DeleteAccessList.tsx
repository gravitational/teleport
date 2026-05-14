import { Alert, ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import { Attempt } from 'shared/hooks/useAttemptNext';

import { AccessListModified } from '../Shared';

export function DeleteAccessList({
  onDelete,
  onCancel,
  attempt,
  accessList,
  fetchRolesError,
}: {
  onDelete(): void;
  onCancel(): void;
  attempt: Attempt;
  accessList: AccessListModified;
  fetchRolesError?: string;
}) {
  const accessListTitle = accessList.title;

  return (
    <>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert>{attempt.statusText}</Alert>}
        {fetchRolesError && <Alert>{fetchRolesError}</Alert>}
        <P1 mb={4}>
          Are you sure you want to delete{' '}
          <Text as="span" bold color="text.main">
            {accessListTitle}
          </Text>
          ?
        </P1>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onDelete}
        >
          Yes, Delete Access List
        </ButtonWarning>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onCancel}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </>
  );
}
