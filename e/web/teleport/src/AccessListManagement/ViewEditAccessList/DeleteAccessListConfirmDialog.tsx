import React from 'react';
import { useHistory } from 'react-router';
import { ButtonSecondary, ButtonWarning, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';

export function DeleteAccessListConfirmDialog({
  accessListName,
  accessListId,
  onClose,
}: {
  accessListName: string;
  accessListId: string;
  onClose(): void;
}) {
  const history = useHistory();

  const { attempt, setAttempt } = useAttempt();
  const isDisabled = attempt.status === 'processing';

  function onOk() {
    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .deleteAccessList(accessListId)
      .then(() => history.push(cfg.getAccessListManagementRoute()))
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  return (
    <Dialog disableEscapeKeyDown={false} onClose={onClose} open={true}>
      <DialogHeader>
        <DialogTitle>Delete Access List?</DialogTitle>
      </DialogHeader>
      <DialogContent width="450px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <Text typography="paragraph" mb="6">
          Are you sure you want to delete{' '}
          <Text as="span" bold color="text.main">
            {accessListName}
          </Text>{' '}
          ?
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Delete Access List
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={onClose}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
