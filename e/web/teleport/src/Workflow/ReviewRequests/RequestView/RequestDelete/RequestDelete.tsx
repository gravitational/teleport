import React from 'react';
import { ButtonWarning, ButtonSecondary, Flex, Text, Alert } from 'design';
import TextSelectCopy from 'teleport/components/TextSelectCopy';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import RolesRequested from '../RolesRequested';
import useTeleportE from 'e-teleport/useTeleportE';
import useRequestDelete, { Props } from './useRequestDelete';

export default function Container(props: Omit<Props, 'ctx'>) {
  const ctx = useTeleportE();
  const state = useRequestDelete({ ...props, ctx });
  return <RequestDelete {...state} />;
}

export function RequestDelete({
  attempt,
  user,
  roles,
  requestId,
  requestState,
  onClose,
  onDelete,
}: ReturnType<typeof useRequestDelete>) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '550px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Delete Request?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'failed' && (
          <Alert kind="danger" children={attempt.statusText} />
        )}
        <Flex flexWrap="wrap" mb={1}>
          <Flex alignItems="baseline">
            <Text mr={1} typography="body2">
              You are about to delete a request from
            </Text>
            <Text mr={1} typography="body2" title={user} bold>
              {user}
            </Text>{' '}
            <Text mr={1} typography="body2">
              for the following roles:
            </Text>
            <RolesRequested roles={roles} />
          </Flex>
        </Flex>
        {requestState === 'APPROVED' && (
          <>
            <Text mt={2} mb={2} typography="body2">
              Since this access request has already been approved, deleting the
              request now will NOT remove the user's access to these roles. If
              you would like to lock the user's access to the requested roles,
              you can run:
            </Text>
            <TextSelectCopy
              mt={2}
              text={`tctl lock --access_request ${requestId}`}
            />
          </>
        )}
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
