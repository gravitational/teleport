/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import styled from 'styled-components';

import Box from 'design/Box';
import { ButtonBorder } from 'design/Button';
import Card from 'design/Card';
import Flex, { Stack } from 'design/Flex';
import * as Icon from 'design/Icon';
import { H1, H2 } from 'design/Text';
import { useStore } from 'shared/libs/stores';

import useTeleport from 'teleport/useTeleport';

/**
 * Renders a scope picker screen that is shown if necessary after the user
 * signs in. The scope picker allows selecting a scope to be pinned to the
 * session.
 *
 * TODO(bl-nero): Actually pick the scope and sign the user into it.
 */
export function LoginScopePicker() {
  const ctx = useTeleport();
  const storeUser = useStore(ctx.storeUser);

  return (
    <Box px="5" width="100%" overflow="scroll">
      <Card my="5" mx="auto" width="100%" maxWidth={600} p={4}>
        <H1>Welcome back, {storeUser.getUsername()}</H1>
        <H2 mt={5} mb={3}>
          Choose a scope for your session:
        </H2>
        <Stack gap={2} as={Ul}>
          {storeUser.getAvailableScopes().map(scope => (
            <Li key={scope}>
              <ScopeButton key={scope} block size="extra-large">
                <Icon.Contract />
                <Flex justifyContent="start" flex={1}>
                  {scope}
                </Flex>
                <SignInAffordance gap={2}>
                  Sign in <Icon.ArrowForward />
                </SignInAffordance>
              </ScopeButton>
            </Li>
          ))}
        </Stack>
      </Card>
    </Box>
  );
}

const Ul = styled.ul`
  list-style: none;
  padding: 0;
  margin: 0;
`;

const Li = styled.li`
  display: block;
  width: 100%;
`;

const ScopeButton = styled(ButtonBorder)`
  justify-content: start;
  gap: ${props => props.theme.space[3]}px;
  font-weight: 400;
  padding-left: ${props => props.theme.space[3]}px;
  padding-right: ${props => props.theme.space[3]}px;
`;

const SignInAffordance = styled(Flex)`
  color: ${props => props.theme.colors.text.muted};
  opacity: 0;
  transition: opacity 0.1s;

  ${ScopeButton}:hover &, ${ScopeButton}:focus-visible & {
    opacity: 100%;
  }
`;
