/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Flex, H3, MenuItem } from 'design';

import { MenuLoginWithActionMenu as MenuLoginWithActionMenuComponent } from './MenuLoginWithActionMenu';

export default {
  title: 'Shared/MenuLoginWithActionMenu',
};

export const MenuLoginWithActionMenu = () => {
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
        <MenuLoginWithActionMenuComponent
          getLoginItems={() => []}
          onSelect={() => null}
          buttonText="Connect"
          size="small"
        >
          {menuItems}
        </MenuLoginWithActionMenuComponent>
      </Example>
      <Example>
        <H3>Processing state</H3>
        <MenuLoginWithActionMenuComponent
          getLoginItems={() => new Promise(() => {})}
          onSelect={() => null}
          buttonText="Connect"
          size="small"
        >
          {menuItems}
        </MenuLoginWithActionMenuComponent>
      </Example>
      <Example>
        <H3>With logins</H3>
        <MenuLoginWithActionMenuComponent
          size="small"
          buttonText="Connect"
          getLoginItems={() => urlItems}
          onSelect={() => null}
          placeholder="Select item to log in..."
          disableSearchAndFilter={true}
        >
          {menuItems}
        </MenuLoginWithActionMenuComponent>
      </Example>
      <Example>
        <H3>With logins and search input</H3>
        <MenuLoginWithActionMenuComponent
          buttonText="Connect"
          size="small"
          getLoginItems={() => loginItems}
          onSelect={() => null}
          placeholder="search login"
        >
          {menuItems}
        </MenuLoginWithActionMenuComponent>
      </Example>
      <Example>
        <H3>Large button with custom width</H3>
        <MenuLoginWithActionMenuComponent
          size="large"
          width="150px"
          buttonText="Connect"
          getLoginItems={() => urlItems}
          onSelect={() => null}
          placeholder="Select item to log in..."
        >
          {menuItems}
        </MenuLoginWithActionMenuComponent>
      </Example>
    </Flex>
  );
};

const Example = styled(Flex).attrs({
  gap: 2,
  flexDirection: 'column',
  alignItems: 'flex-start',
})``;

const makeLoginItem = (login: string) => ({ url: '', login });

const urlItems = [
  'https://portal.azure.com/xxxxx2432-xxx-xx-xx-cxxxxx37ded',
  'https://portal.azure.com/xxxxx2432-xxx-xx-xx-cxxxxx37ded&login_hint=user@example.com',
  'https://myapplications.microsoft.com/?tenantid=xxxxx2432-xxx-xx-xx-cxxxxx37ded',
].map(makeLoginItem);

const loginItems = ['alice', 'bob', 'hector'].map(makeLoginItem);

const menuItems = (
  <>
    <MenuItem onClick={() => alert('Foo')}>Foo</MenuItem>
    <MenuItem as="a" href="https://example.com" target="_blank">
      Link to example.com
    </MenuItem>
    <MenuItem onClick={() => alert('Lorem ipsum dolor sit amet')}>
      Lorem ipsum dolor sit amet
    </MenuItem>
  </>
);
