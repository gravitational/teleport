import { useState } from 'react';
import { Box, Input, Text } from 'design';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { Attempt } from 'shared/hooks/useAsync';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';

type Props = {
  lockType: string;
  targetId: string;
  createLock: (
    lockType: string,
    targetId: string,
    message: string,
    ttl: string
  ) => Promise<[void, Error]>;
  createLockAttempt: Attempt<void>;
};

export const CreateLockButton = ({
  createLock,
  createLockAttempt,
  lockType,
  targetId,
}: Props) => {
  const [open, setOpen] = useState(false);
  const [message, setMessage] = useState('');
  const [ttl, setTtl] = useState('');

  return (
    <>
      <ButtonPrimary onClick={() => setOpen(true)}>Lock</ButtonPrimary>
      <Dialog open={open}>
        <DialogHeader>
          <Text typography="h4" color="text.primary" bold>
            Create Lock for {targetId}
          </Text>
        </DialogHeader>
        <DialogContent
          css={`
            min-width: 400px;
          `}
        >
          <Box mt={3}>
            <Text mr={2}>Message</Text>
            <Input
              placeholder={`Going down for maintenance`}
              value={message}
              onChange={e => setMessage(e.currentTarget.value)}
            />
          </Box>
          <Box mt={3}>
            <Text mr={2}>TTL</Text>
            <Input
              placeholder={`2h45m, 5h, empty=never`}
              value={ttl}
              onChange={e => setTtl(e.currentTarget.value)}
            />
          </Box>
          <DialogFooter mt={5}>
            <ButtonPrimary
              width="100%"
              onClick={() => createLock(lockType, targetId, message, ttl)}
              disabled={createLockAttempt.status === 'processing'}
            >
              Create Lock
            </ButtonPrimary>
            <ButtonSecondary mt={4} onClick={() => setOpen(false)} width="100%">
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
};
