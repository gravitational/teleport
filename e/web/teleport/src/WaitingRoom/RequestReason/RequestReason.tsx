import React from 'react';
import session from 'teleport/services/websession';
import {
  ButtonPrimary,
  ButtonSecondary,
  Text,
  LabelInput,
  Alert,
  Box,
} from 'design';

import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import useRequestReason, { Props } from './useRequestReason';

export default function Container({ onCreateRequest, prompt }: Props) {
  const state = useRequestReason({ onCreateRequest, prompt });

  return <RequestReason {...state} />;
}

export function RequestReason(props: ReturnType<typeof useRequestReason>) {
  const { attempt, reason, setReason, createRequest, prompt } = props;
  const requestPrompt = prompt
    ? prompt
    : 'To access your Teleport account, please send an authorization request using the form below.';

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Request Access</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.isFailed && <Alert kind="danger" children={attempt.message} />}
        <Text mb={4}>{requestPrompt}</Text>
        <LabelInput mb={1}>Authorization Request Message</LabelInput>
        <Box
          height="100px"
          as="textarea"
          p={2}
          borderRadius={2}
          placeholder="Describe your request..."
          value={reason}
          onChange={e => setReason(e.target.value)}
          autoFocus
          style={{ outline: 'none' }}
        />
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          disabled={attempt.isProcessing || reason.trim().length === 0}
          onClick={createRequest}
        >
          Send Request
        </ButtonPrimary>
        <ButtonSecondary onClick={() => session.logout()}>
          Cancel & Logout
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
