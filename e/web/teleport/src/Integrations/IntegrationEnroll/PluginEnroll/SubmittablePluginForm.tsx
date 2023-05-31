import React, { FormEvent } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import { ButtonPrimary, ButtonSecondary, Box, Flex, Text } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';
import Validation, { Validator } from 'shared/components/Validation';
import { getXCSRFToken } from 'teleport/services/api';

import cfg from 'e-teleport/config';

import { HostedPlugin } from './plugins';

// SubmittablePluginForm is a form that uses the default form submit event.
export function SubmittablePluginForm({
  plugin,
  eventId,
}: {
  plugin: HostedPlugin;
  eventId: string;
}) {
  function onSubmit(validator: Validator, e: FormEvent) {
    if (!validator.validate()) {
      e.preventDefault();
    }
  }

  return (
    <Box mt={3}>
      <Text fontWeight="bold" typography="h4">
        {plugin.fullName}
      </Text>
      {plugin.Description && <plugin.Description />}
      {plugin.permissions?.length && (
        <Flex gap={6} p={4} bg="levels.surface">
          {plugin.permissions.map((perm, index) => (
            <Box key={index}>
              <Text fontWeight="bold" typography="h6">
                {perm.category}
              </Text>
              <PermissionList>
                {perm.permissions.map(p => (
                  <PermissionListItem key={`${index}${p.title}`}>
                    {p.title}{' '}
                    {p.description && (
                      <ToolTipInfo>{p.description}</ToolTipInfo>
                    )}
                  </PermissionListItem>
                ))}
              </PermissionList>
            </Box>
          ))}
        </Flex>
      )}
      <Box mt={3}>
        <Validation>
          {/* A "normal" HTTP form is used here instead of an AJAX request,
          since the user needs to be redirected to the OAuth provider after submitting. */}
          {({ validator }) => (
            <form
              action={cfg.getPluginUrl()}
              onSubmit={e => onSubmit(validator, e)}
              method="POST"
            >
              <input type="hidden" name="csrf_token" value={getXCSRFToken()} />
              <input type="hidden" name="event_id" value={eventId} />
              {/* TODO: make this a proper input and validate against existing instances
          when we start allowing multiple instances per type */}
              <input
                type="hidden"
                name="name"
                value={`${plugin.type}-default`}
              />
              <input type="hidden" name="type" value={plugin.type} />
              <Flex flexDirection="column">
                <Flex>{plugin.FormMixin && <plugin.FormMixin />}</Flex>
                <Flex gap={2}>
                  <ButtonPrimary type="submit" mr={3}>
                    Connect {plugin.name}
                  </ButtonPrimary>
                  <ButtonSecondary
                    as={Link}
                    to={cfg.oss.getIntegrationEnrollRoute()}
                  >
                    Back
                  </ButtonSecondary>
                </Flex>
              </Flex>
            </form>
          )}
        </Validation>
      </Box>
    </Box>
  );
}

const PermissionList = styled.ul`
  list-style: none;
  padding: 0;
`;

const PermissionListItem = styled.li`
  margin-top: ${({ theme }) => theme.space[2]}px;
`;
