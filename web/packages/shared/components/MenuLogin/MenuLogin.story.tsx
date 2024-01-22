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

import React from 'react';
import { Flex } from 'design';

import { MenuLogin } from './MenuLogin';
import { MenuLoginHandle } from './types';

export default {
  title: 'Shared/MenuLogin',
};

export const MenuLoginStory = () => <MenuLoginExamples />;

function MenuLoginExamples() {
  return (
    <Flex
      width="400px"
      height="100px"
      alignItems="center"
      justifyContent="space-around"
      bg="levels.surface"
    >
      <MenuLogin
        getLoginItems={() => []}
        onSelect={() => null}
        placeholder="Please provide user name…"
      />
      <MenuLogin
        getLoginItems={() => new Promise(() => {})}
        placeholder="MenuLogin in processing state"
        onSelect={() => null}
      />
      <SampleMenu />
    </Flex>
  );
}

class SampleMenu extends React.Component {
  menuRef = React.createRef<MenuLoginHandle>();

  componentDidMount() {
    this.menuRef.current.open();
  }

  render() {
    return (
      <MenuLogin
        ref={this.menuRef}
        getLoginItems={() => loginItems}
        onSelect={() => null}
      />
    );
  }
}

const loginItems = ['root', 'jazrafiba', 'evubale', 'ipizodu'].map(login => ({
  url: '',
  login,
}));
