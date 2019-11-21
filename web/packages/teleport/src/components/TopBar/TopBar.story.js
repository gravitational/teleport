import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { storiesOf } from '@storybook/react';
import * as Icons from 'design/Icon';
import { DashboardTopNav } from './TopBar';

storiesOf('Teleport/TopBar', module).add('TopBar', () => {
  const history = createMemoryHistory({
    initialEntries: ['/web/page1'],
    initialIndex: 0,
  });

  const props = {
    ...defaultProps,
  };
  return (
    <Router history={history}>
      <DashboardTopNav height="40px" {...props} />
    </Router>
  );
});

const defaultProps = {
  version: '1.1.1',
  username: 'john@example.com',
  topMenuItems: [
    {
      Icon: Icons.User,
      to: '/web/page1',
      title: 'Page1',
    },
  ],
};
