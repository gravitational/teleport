import React from 'react';
import { storiesOf } from '@storybook/react';
import { Hub } from './Hub';
import { Router } from 'react-router';
import { createMemoryHistory } from 'history';
import cfg from 'e-gravity/config';
import FeatureLicense from './features/featureHubLicenses';

storiesOf('GravityHub', module)
  .add('Layout', () => {
    const history = createMemoryHistory({
      initialEntries: [cfg.routes.hubLicenses],
    });

    const props = {

      attempt: {
        isSuccess: true,
      },

      features: [ new FeatureLicense() ],

      navItems: [{
        title: 'Licenses',
        to: cfg.routes.hubLicenses
      }],

      userName: 'itibo@wok.om',
    }

    return (
      <Router history={history}>
        <Hub {...props} />
      </Router>
    );
  });