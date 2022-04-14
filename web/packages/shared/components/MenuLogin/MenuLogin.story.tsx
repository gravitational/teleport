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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';

storiesOf('Shared/MenuLogin', module).add('MenuLogin', () => (
  <Flex
    width="400px"
    height="100px"
    alignItems="center"
    justifyContent="space-around"
    bg="primary.light"
  >
    <MenuLogin
      getLoginItems={() => []}
      onSelect={() => null}
      placeholder="Please provide user name…"
    />
    <SampleMenu />
  </Flex>
));

class SampleMenu extends React.Component {
  menuRef = React.createRef<MenuLogin>();

  componentDidMount() {
    this.menuRef.current.onOpen();
  }

  render() {
    return (
      <Router history={createMemoryHistory()}>
        <MenuLogin
          ref={this.menuRef}
          getLoginItems={() => loginItems}
          onSelect={() => null}
        />
      </Router>
    );
  }
}

const loginItems = ['root', 'jazrafiba', 'evubale', 'ipizodu'].map(login => ({
  url: '',
  login,
}));
