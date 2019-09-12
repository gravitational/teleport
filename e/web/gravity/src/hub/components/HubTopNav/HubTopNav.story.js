import React from 'react';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import { storiesOf } from '@storybook/react'
import HubTopNav from './HubTopNav';

storiesOf('GravityHub/HubTopNav', module)
  .add('HubTopNav', () => {

    const history = createMemoryHistory({
      initialEntries: ['/web/page1'],
      initialIndex: 0
    });

    const props = {
      ...defaultProps,
    }
    return (
      <Router history={history}>
        <HubTopNav height="40px" {...props}/>
      </Router>
    )}
  );

const defaultProps = {
  userName: 'john@example.com',
  items: [
    {
      to: '/web/page1',
      title: 'Page1'
    },
    {
      to: '/web/page2',
      title: 'Page2'
    },
    {
      to: '/web/page3',
      title: 'Page3'
    },
  ]
}