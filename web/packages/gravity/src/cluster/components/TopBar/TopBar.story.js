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

import $ from 'jQuery';
import React from 'react'
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { storiesOf } from '@storybook/react'
import { withInfo } from '@storybook/addon-info'
import { TopBar } from './TopBar'
import * as Icons from 'design/Icon';
import { StatusEnum } from 'gravity/services/clusters';

storiesOf('Gravity/TopBar', module)
  .addDecorator(withInfo)
  .add('Healthy', () => {
    const newProps = {
      ...props,
      info: {
        ...props.info,
        status: StatusEnum.READY
      }
    }
    return (
      <Router history={inMemoryHistory}>
        <TopBar {...newProps}/>
      </Router>
    )}
  )
  .add('In Progress', () => {
    const newProps = {
      ...props,
      info: {
        ...props.info,
        status: StatusEnum.PROCESSING
      }
    }
    return (
      <Router history={inMemoryHistory}>
        <TopBar {...newProps}/>
      </Router>
    )
  })
  .add('With Issues', () => {
    const newProps = {
      ...props,
      info: {
        ...props.info,
        status: StatusEnum.ERROR
      }
    }

    return (
      <Router history={inMemoryHistory}>
        <TopBar {...newProps}/>
      </Router>
    )
  }
);

const props = {
  user: {
    userId: 'john@example.com'
  },
  navItems: [],
  info: {
    status: {},
    publicUrls: ['http://localhost/'],
    internalUrls: [],
    commands: {}
  },
  onRefresh: () => $.Deferred().resolve(),
  menu: [{
    Icon: Icons.User,
    title: 'Menu Item',
    to: 'xxx'
  }]
}

const inMemoryHistory = createMemoryHistory({ });