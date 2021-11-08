import React from 'react';
import { Indicator, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, { DialogHeader, DialogTitle, DialogFooter } from 'design/Dialog';
import { Danger } from 'design/Alert';
import useRecoveryCodesDialog, { State, Props } from './useRecoveryCodesDialog';
import RecoveryCodes from 'e-teleport/components/RecoveryCodes';
import useTeleportE from 'e-teleport/useTeleportE';

export default function Container(props: Props) {
  const ctx = useTeleportE();
  const state = useRecoveryCodesDialog(ctx, props);
  return <RecoveryCodesDialog {...state} />;
}

export function RecoveryCodesDialog({
  attempt,
  recoveryCodes,
  generateCodes,
  close,
  closeWithDateRefresh,
  isNewCodes,
}: State) {
  if (attempt.status === 'failed') {
    return (
      <Dialog
        dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
        onClose={close}
        disableEscapeKeyDown={false}
        open={true}
      >
        <DialogHeader>
          <DialogTitle>An error has occurred</DialogTitle>
        </DialogHeader>
        <Danger>{attempt.statusText}</Danger>
        <DialogFooter mt={3}>
          <ButtonPrimary onClick={generateCodes} mr={2}>
            Try again
          </ButtonPrimary>
          <ButtonSecondary onClick={close}>Close</ButtonSecondary>
        </DialogFooter>
      </Dialog>
    );
  }

  return (
    <Dialog
      dialogCss={() => ({
        padding: '0px',
        background: 'none',
        ...(attempt.status === 'processing' && { boxShadow: 'none' }),
      })}
      onClose={closeWithDateRefresh}
      open={true}
    >
      {attempt.status === 'success' && (
        <RecoveryCodes
          recoveryCodes={recoveryCodes}
          redirect={closeWithDateRefresh}
          continueText={'Close dialog'}
          isNewCodes={isNewCodes}
        />
      )}
      {attempt.status === 'processing' && <Indicator />}
    </Dialog>
  );
}
