/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { storiesOf } from '@storybook/react';
import { Flex } from 'design';

import { MenuLogin } from './MenuLogin';
import { MenuLoginHandle } from './types';

storiesOf('Shared/MenuLogin', module).add('MenuLogin', () => {
  return <MenuLoginExamples />;
});

export function MenuLoginExamples() {
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
