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
import { useToastNotifications } from 'shared/components/ToastNotification';
import Validation, { Validator } from 'shared/components/Validation';
import { useAttemptNext } from 'shared/hooks';

import useTeleport from 'e-teleport/useTeleportE';

import { createSuccessNotification } from './common';
import { InviteCollaboratorsForm } from './InviteCollaboratorsForm';
import { InviteCollaboratorsDialogProps, RoleOption } from './types';

const ClusterName = styled.span<{
  theme: any;
}>`
  color: ${props => props.theme.colors.text.main};
  text-decoration-line: underline;
  text-decoration-style: dashed;
  text-decoration-color: ${props => props.theme.colors.text.main};
`;

export function InviteCollaboratorsDialog({
  onClose,
}: InviteCollaboratorsDialogProps) {
  const ctx = useTeleport();
  const toastNotification = useToastNotifications();

  const clusterId = ctx.storeUser.getClusterId();
  const { attempt: loadAttempt, run: runLoad } = useAttemptNext('');
  const [users, setUsers] = useState<Set<string>>(() => new Set());
  const [recipientsValue, setRecipientsValue] = useState<Option[]>([]);
  const [selectedRoles, setSelectedRoles] = useState<RoleOption[]>([]);
  const { attempt: submitAttempt, setAttempt: setSubmitAttempt } =
    useAttemptNext('');

  useEffect(() => {
    // TODO(rudream): Refactor this logic to not require fetching all users up front
    function fetchUsers(): Promise<Set<string>> {
      if (ctx.getFeatureFlags().users) {
        return ctx.userService
          .fetchAllUsers()
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
        toastNotification.add(createSuccessNotification(invite));
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
