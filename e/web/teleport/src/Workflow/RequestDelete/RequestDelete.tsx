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
        <Flex flexWrap="wrap" mb={2}>
          <Flex alignItems="baseline" mb={2}>
            <Text mr={1} typography="body2">
              You are about to delete a request from
            </Text>
            <Text mr={1} typography="body2" title={user} bold>
              {user}
            </Text>{' '}
            <Text mr={1} typography="body2">
              for the roles:
            </Text>
            <RolesRequested roles={roles} />
          </Flex>
        </Flex>
        <Text mb={3} typography="body2">
          If this request has been approved, deleting the request will not
          remove this users access to these roles.
        </Text>
        <Text typography="body2">
          {' '}
          If you would also like to lock this users access run:
        </Text>
        <TextSelectCopy
          mt={2}
          text={`tctl lock --access_request ${requestId}`}
        />
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
