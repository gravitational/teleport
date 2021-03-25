import React from 'react';
import { ButtonWarning, ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import useTeleportE from 'e-teleport/useTeleportE';
import useRequestDelete, { Props } from './useRequestDelete';

export default function Container(props: Omit<Props, 'ctx'>) {
  const ctx = useTeleportE();
  const state = useRequestDelete({ ...props, ctx });
  return <RequestDelete {...state} />;
}

export function RequestDelete({
  attempt,
  requestId,
  onClose,
  onDelete,
}: ReturnType<typeof useRequestDelete>) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Delete Request?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        <Text mb={4} mt={1}>
          You are about to delete request
          <Text bold as="span">
            {` ${requestId}`}
          </Text>
          .
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          mr="3"
          disabled={attempt.status === 'processing'}
          onClick={onDelete}
        >
          Delete Request
        </ButtonWarning>
        <ButtonSecondary
          disabled={attempt.status === 'processing'}
          onClick={onClose}
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
