/*
Copyright 2019-2020 Gravitational, Inc.

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
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import * as Icons from 'design/Icon';
import { DashboardTopNav } from './TopBar';

export default {
  title: 'Teleport/TopBar',
};

export function TopBar() {
  const props = {
    ...defaultProps,
  };
  return (
    <Router history={createMemoryHistory()}>
      <DashboardTopNav height="40px" {...props} />
    </Router>
  );
}

const defaultProps = {
  username: 'john@example.com',
  topMenuItems: [
    {
      Icon: Icons.User,
      to: '/web/page1',
      title: 'Page1',
    },
  ],
};
