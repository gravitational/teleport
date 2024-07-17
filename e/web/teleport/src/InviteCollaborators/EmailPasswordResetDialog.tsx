import React from 'react';
import styled from 'styled-components';

import { ButtonPrimary, Text, Flex, ButtonSecondary, Box, H2 } from 'design';
import * as Alerts from 'design/Alert';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';
import { PaperPlane, UserAdd } from 'design/Icon';
import { requiredEmailLike } from 'shared/components/Validation/rules';
import { useAttemptNext } from 'shared/hooks';
import { Attempt } from 'shared/hooks/useAttemptNext';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

import useTeleport from 'e-teleport/useTeleportE';

import { EmailPasswordResetDialogProps } from './types';

const UserStyle = styled.span`
  text-decoration-line: underline;
  text-decoration-style: dashed;
`;

function ResetSuccess({ onClose, username }: EmailPasswordResetDialogProps) {
  return (
    <>
      <DialogContent>
        <Text>
          <UserStyle>{username}</UserStyle> has been sent a new cluster
          invitation link via email.
        </Text>
      </DialogContent>

      <DialogFooter css={{ display: 'flex' }}>
        <ButtonPrimary onClick={() => onClose()} block={true} size="large">
          Close
        </ButtonPrimary>
      </DialogFooter>
    </>
  );
}

type ResetFormProps = EmailPasswordResetDialogProps & {
  submitAttempt: Attempt;
  setSubmitAttempt: (Attempt) => void;
};

function ResetForm({
  onClose,
  username,
  submitAttempt,
  setSubmitAttempt,
}: ResetFormProps) {
  const ctx = useTeleport();

  const isEmailLike = requiredEmailLike(username)().valid;

  const handleSendClick = async (
    e: React.MouseEvent<HTMLButtonElement>
  ): Promise<void> => {
    e.preventDefault();

    if (!isEmailLike) {
      setSubmitAttempt({
        status: 'failed',
        statusText: `${username} does not appear to be a valid email address.`,
      });
      return;
    }

    setSubmitAttempt({ status: 'processing' });

    ctx.cloudService
      .sendTeleportCredentialReset({
        recipient: username,
      })
      .then(() => {
        setSubmitAttempt({ status: 'success' });
      })
      .catch(err => {
        setSubmitAttempt({ status: 'failed', statusText: err.message });
      });
  };

  return (
    <>
      <DialogContent>
        {submitAttempt.status === 'failed' && (
          <Alerts.Danger children={submitAttempt.statusText} />
        )}
        {isEmailLike && (
          <Text>
            This will send <UserStyle>{username}</UserStyle> a new cluster
            invitation link via email.
          </Text>
        )}
        {!isEmailLike && (
          <Alerts.Danger mb={0}>
            <Text>
              User <UserStyle>{username}</UserStyle> does not appear to have an
              email-like name so their credentials cannot be reset from this
              interface. You may enroll them as a new user via email, or
              generate a credential reset link using the{' '}
              <Text as="span" bold>
                <code>tctl</code>{' '}
              </Text>{' '}
              CLI:
              <Box>
                <TextSelectCopy
                  mt={2}
                  text={`tctl users reset "${username}"`}
                />
              </Box>
            </Text>
          </Alerts.Danger>
        )}
      </DialogContent>

      <DialogFooter css={{ display: 'flex' }}>
        <ButtonPrimary
          mr={4}
          width="45%"
          disabled={!isEmailLike || submitAttempt.status === 'processing'}
          onClick={handleSendClick}
          block={true}
          size="large"
        >
          <PaperPlane size="medium" mr={2} />
          Send New Invite
        </ButtonPrimary>
        <ButtonSecondary
          width="45%"
          disabled={submitAttempt.status === 'processing'}
          onClick={() => onClose()}
          block={true}
          size="large"
        >
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </>
  );
}

export default function EmailPasswordResetDialog({
  onClose,
  username,
}: EmailPasswordResetDialogProps) {
  const { attempt: submitAttempt, setAttempt: setSubmitAttempt } =
    useAttemptNext('');

  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '500px',
        width: '100%',
        overflow: 'initial',
      })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <Flex alignItems="center">
          <Flex mr={3} justifyContent="center" width="24px">
            <UserAdd />
          </Flex>
          <Box>
            <H2>Reset User Credentials</H2>
            <Text color="text.slightlyMuted">
              Send <UserStyle>{username}</UserStyle> a new cluster invitation to
              reset their credentials.
            </Text>
          </Box>
        </Flex>
      </DialogHeader>

      {submitAttempt.status !== 'success' && (
        <ResetForm
          username={username}
          onClose={onClose}
          submitAttempt={submitAttempt}
          setSubmitAttempt={setSubmitAttempt}
        />
      )}

      {submitAttempt.status === 'success' && (
        <ResetSuccess username={username} onClose={onClose} />
      )}
    </Dialog>
  );
}
