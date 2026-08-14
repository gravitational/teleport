import React, { useState } from 'react';

import { Box, ButtonPrimary, ButtonSecondary, Flex, H2, Text } from 'design';
import { PaperPlane, UserAdd } from 'design/Icon';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';

import { InviteCollaboratorsForm } from 'e-teleport/InviteCollaborators/InviteCollaboratorsForm';
import { RoleOption } from 'e-teleport/InviteCollaborators/types';
import cfg from 'teleport/config';
import api from 'teleport/services/api';
import { Resource } from 'teleport/services/resources';
import {
  CloudUserInvites,
  storageService,
} from 'teleport/services/storageService';

async function fetchPresetRoles(): Promise<Array<Resource<'role'>>> {
  return api.get(cfg.getPresetRolesUrl());
}

export function InviteCollaboratorsCard({
  onSubmit,
}: {
  onSubmit: () => void;
}) {
  const [users] = useState<Set<string>>(() => new Set());
  const [recipientsValue, setRecipientsValue] = useState<Option[]>([]);
  const [selectedRoles, setSelectedRoles] = useState<RoleOption[]>([]);

  async function fetchRoles(input: string): Promise<RoleOption[]> {
    const roles = await fetchPresetRoles();
    return roles
      .filter(role => role.name.includes(input))
      .map(role => ({
        label: role.name,
        value: {
          name: role.name,
          description: role.description,
        },
      }));
  }

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
              <H2>Invite Users</H2>
              <Text color="text.slightlyMuted">Collaborate with your team</Text>
            </Box>
          </Flex>
          <InviteCollaboratorsForm
            users={users}
            fetchRoles={fetchRoles}
            recipientsValue={recipientsValue}
            setRecipientsValue={setRecipientsValue}
            selectedRoles={selectedRoles}
            setSelectedRoles={setSelectedRoles}
          />
          <Box>
            <ButtonPrimary
              mr={4}
              width="100%"
              onClick={e => handleInviteClick(e, validator)}
              block={true}
              size="large"
              mb={2}
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
