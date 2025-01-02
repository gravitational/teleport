import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Indicator,
  Text,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/Dialog';
import { PaperPlane, UserAdd } from 'design/Icon';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { useAttemptNext } from 'shared/hooks';

import useTeleport from 'e-teleport/useTeleportE';

import { createSuccessNotification } from './common';
import { InviteCollaboratorsForm } from './InviteCollaboratorsForm';
import {
  NotificationEntry,
  NotificationItem,
  Notifications,
} from './Notifications';
import { InviteCollaboratorsDialogProps, RoleOption } from './types';

const ClusterName = styled.span<{
  theme: any;
}>`
  color: ${props => props.theme.colors.text.main};
  text-decoration-line: underline;
  text-decoration-style: dashed;
  text-decoration-color: ${props => props.theme.colors.text.main};
`;

function InviteCollaboratorsDialogInner({
  onClose,
  addNotification,
}: WrappedProps) {
  const ctx = useTeleport();

  const clusterId = ctx.storeUser.getClusterId();
  const { attempt: loadAttempt, run: runLoad } = useAttemptNext('');
  const [users, setUsers] = useState<Set<string>>(() => new Set());
  const [recipientsValue, setRecipientsValue] = useState<Option[]>([]);
  const [selectedRoles, setSelectedRoles] = useState<RoleOption[]>([]);
  const { attempt: submitAttempt, setAttempt: setSubmitAttempt } =
    useAttemptNext('');

  useEffect(() => {
    function fetchUsers(): Promise<Set<string>> {
      if (ctx.getFeatureFlags().users) {
        return ctx.userService
          .fetchUsers()
          .then(users => new Set(users.map(u => u.name.toLowerCase())));
      }

      return Promise.resolve(new Set());
    }

    runLoad(() =>
      Promise.all([fetchUsers()]).then(values => {
        setUsers(values[0]);
      })
    );
  }, []);

  async function fetchRoles(input: string): Promise<RoleOption[]> {
    if (ctx.getFeatureFlags().roles) {
      const roles = await ctx.resourceService.fetchRoles({
        search: input,
      });
      return roles.items.map(role => ({
        label: role.name,
        value: {
          name: role.name,
          description: role.description,
        },
      }));
    }
  }

  const handleSendClick = async (
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ): Promise<void> => {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    setSubmitAttempt({ status: 'processing' });

    const invite = {
      recipients: recipientsValue.map(r => r.value),
      roles: selectedRoles.map(r => r.value.name),
    };

    ctx.cloudService
      .sendTeleportInvite(invite)
      .then(() => {
        onClose();
        addNotification(createSuccessNotification(invite));
      })
      .catch(err => {
        setSubmitAttempt({ status: 'failed', statusText: err.message });
      });
  };

  return (
    <Validation>
      {({ validator }) => (
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
                <H2>Invite Users</H2>
                <Text color="text.slightlyMuted">
                  Collaborate with your team on{' '}
                  <ClusterName>{clusterId}</ClusterName>.
                </Text>
              </Box>
            </Flex>
          </DialogHeader>

          <DialogContent>
            {loadAttempt.status == 'processing' && (
              <Box textAlign="center">
                <Indicator />
              </Box>
            )}
            {loadAttempt.status === 'failed' && (
              <Alert kind="danger" children={loadAttempt.statusText} />
            )}

            {submitAttempt.status === 'failed' && (
              <Alert kind="danger" children={submitAttempt.statusText} />
            )}

            {loadAttempt.status !== 'failed' && (
              <InviteCollaboratorsForm
                users={users}
                fetchRoles={fetchRoles}
                recipientsValue={recipientsValue}
                setRecipientsValue={setRecipientsValue}
                selectedRoles={selectedRoles}
                setSelectedRoles={setSelectedRoles}
                onClose={onClose}
                hidden={loadAttempt.status != 'success'}
              />
            )}
          </DialogContent>

          <DialogFooter css={{ display: 'flex' }}>
            <ButtonPrimary
              mr={4}
              width="45%"
              disabled={submitAttempt.status === 'processing'}
              onClick={e => handleSendClick(e, validator)}
              block={true}
              size="large"
            >
              <PaperPlane size="medium" mr={2} />
              Send Invitation
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
        </Dialog>
      )}
    </Validation>
  );
}

type WrappedProps = InviteCollaboratorsDialogProps & {
  addNotification: (item: NotificationEntry) => void;
};

/**
 * A wrapper component for the InviteCollaboratorsDialog that adds toast
 * notifications.
 */
export function InviteCollaboratorsDialog(
  props: InviteCollaboratorsDialogProps
) {
  // We need to generate unique notification IDs, but don't particularly care
  // what they are. We'll just use an incremental counter.
  const [count, setCount] = useState<number>(0);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);

  function addNotification(item: NotificationEntry) {
    setNotifications([
      {
        ...item,
        id: count.toString(),
        dismissAfterMs: item.dismissAfterMs,
      },
      ...notifications,
    ]);
    setCount(count + 1);
  }

  function dismiss(id: string) {
    setNotifications(notifications.filter(i => i.id != id));
  }

  const wrappedProps = {
    ...props,
    addNotification,
  };

  // Note: we control dialog open state here to make sure state is fully reset
  // when the dialog is closed and reopened.
  return (
    <>
      {props.open && <InviteCollaboratorsDialogInner {...wrappedProps} />}
      <Notifications items={notifications} dismiss={dismiss} />
    </>
  );
}
