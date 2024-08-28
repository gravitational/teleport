/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { useEffect, useRef } from 'react';
import { Flex, H3 } from 'design';
import styled from 'styled-components';

import { MenuLogin } from './MenuLogin';
import { MenuLoginHandle } from './types';

export default {
  title: 'Shared/MenuLogin',
};

export const MenuLoginStory = () => <MenuLoginExamples />;

function MenuLoginExamples() {
  return (
    <Flex
      inline
      p={4}
      gap="128px"
      justifyContent="flex-start"
      bg="levels.surface"
    >
      <Example>
        <H3>No logins</H3>
        <MenuLogin
          getLoginItems={() => []}
          onSelect={() => null}
          placeholder="Please provide user name…"
        />
      </Example>
      <Example>
        <H3>Processing state</H3>
        <MenuLogin
          getLoginItems={() => new Promise(() => {})}
          placeholder="MenuLogin in processing state"
          onSelect={() => null}
        />
      </Example>
      <Example>
        <H3>With logins</H3>
        <SampleMenu />
      </Example>
    </Flex>
  );
}

const Example = styled(Flex).attrs({
  gap: 2,
  flexDirection: 'column',
  alignItems: 'flex-start',
})``;

const SampleMenu = () => {
  const menuRef = useRef<MenuLoginHandle>();

  useEffect(() => {
    menuRef.current.open();
  }, []);

  return (
    <MenuLogin
      ref={menuRef}
      getLoginItems={() => loginItems}
      onSelect={() => null}
    />
  );
};

const loginItems = ['root', 'jazrafiba', 'evubale', 'ipizodu'].map(login => ({
  url: '',
  login,
}));
