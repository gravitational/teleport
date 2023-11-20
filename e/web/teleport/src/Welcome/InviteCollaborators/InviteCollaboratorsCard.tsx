import React, { useEffect, useState } from 'react';
import { useTheme } from 'styled-components';

import {
  ButtonPrimary,
  Text,
  Flex,
  ButtonSecondary,
  Alert,
  Box,
  Indicator,
} from 'design';
import { PaperPlane, UserAdd } from 'design/Icon';

import { Option } from 'shared/components/Select';
import { useAttemptNext } from 'shared/hooks';
import Validation, { Validator } from 'shared/components/Validation';

import api from 'teleport/services/api';
import cfg from 'teleport/config';
import {
  CloudUserInvites,
  storageService,
} from 'teleport/services/storageService';
import { Resource } from 'teleport/services/resources';

import { InviteCollaboratorsForm } from 'e-teleport/InviteCollaborators/InviteCollaboratorsForm';
import { RoleOption } from 'e-teleport/InviteCollaborators/types';

async function fetchPresetRoles(): Promise<Array<Resource<'role'>>> {
  return api.get(cfg.getPresetRolesUrl());
}

export function InviteCollaboratorsCard({
  onSubmit,
}: {
  onSubmit: () => void;
}) {
  const theme = useTheme();

  const { attempt: loadAttempt, run: runLoad } = useAttemptNext('processing');
  const [users] = useState<Set<string>>(() => new Set());
  const [roles, setRoles] = useState<RoleOption[]>([]);
  const [recipientsValue, setRecipientsValue] = useState<Option[]>([]);
  const [selectedRoles, setSelectedRoles] = useState<RoleOption[]>([]);

  useEffect(() => {
    function fetchRoles(): Promise<RoleOption[]> {
      return fetchPresetRoles().then(roles => {
        return roles.map(role => ({
          label: role.name,
          value: {
            name: role.name,
            description: role.description,
          },
        }));
      });
    }

    runLoad(() => fetchRoles().then(setRoles));
  }, []);

  function handleInviteClick(
    e: React.MouseEvent<HTMLButtonElement>,
    validator: Validator
  ) {
    e.preventDefault();
    if (!validator.validate()) {
      return;
    }

    const invite: CloudUserInvites = {
      recipients: recipientsValue.map(r => r.label),
      roles: selectedRoles.map(r => r.value.name),
    };

    storageService.setCloudUserInvites(invite);
    onSubmit();
  }

  return (
    <Validation>
      {({ validator }) => (
        <>
          <Flex alignItems="center" mb={4}>
            <Flex mr={3} justifyContent="center" width="24px">
              <UserAdd />
            </Flex>
            <Box>
              <Text typography="h3" color={theme.colors.text.main}>
                Invite Users
              </Text>
              <Text color="text.slightlyMuted">Collaborate with your team</Text>
            </Box>
          </Flex>
          {loadAttempt.status == 'processing' && (
            <Box textAlign="center" m={10}>
              <Indicator />
            </Box>
          )}
          {loadAttempt.status == 'failed' && (
            <Alert kind="danger" children={loadAttempt.statusText} />
          )}
          {loadAttempt.status !== 'failed' && (
            <InviteCollaboratorsForm
              users={users}
              roles={roles}
              recipientsValue={recipientsValue}
              setRecipientsValue={setRecipientsValue}
              selectedRoles={selectedRoles}
              setSelectedRoles={setSelectedRoles}
              hidden={loadAttempt.status != 'success'}
            />
          )}
          <Box>
            <ButtonPrimary
              mr={4}
              width="100%"
              onClick={e => handleInviteClick(e, validator)}
              block={true}
              size="large"
              mb={2}
              disabled={loadAttempt.status == 'processing'}
            >
              <PaperPlane size="medium" mr={2} />
              Invite
            </ButtonPrimary>
            <ButtonSecondary
              width="100%"
              onClick={() => onSubmit()}
              block={true}
              size="large"
            >
              Skip
            </ButtonSecondary>
          </Box>
        </>
      )}
    </Validation>
  );
}
