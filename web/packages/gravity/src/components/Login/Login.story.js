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
import LoginSuccess from './LoginSuccess';
import LoginFailed from './LoginFailed';
import { Login } from './Login';
import longLogoSvg from 'design/assets/images/sample-logo-long.svg';
import squireLogoSvg from 'design/assets/images/sample-logo-squire.svg';

storiesOf('Gravity/Login', module)
  .add('Login', () => {
    const props = {
      authType: '',
      attempt: {},
      auth2faType: 'off',
    };

    return <Login {...props} />;
  })
  .add('Long Logo', () => {
    const props = {
      authType: '',
      attempt: {},
      auth2faType: 'off',
      logoSrc: longLogoSvg,
    };

    return <Login {...props} />;
  })
  .add('Square Logo', () => {
    const props = {
      authType: '',
      attempt: {},
      auth2faType: 'off',
      logoSrc: squireLogoSvg,
    };

    return <Login {...props} />;
  })
  .add('LoginSuccess', () => {
    return <LoginSuccess />;
  })
  .add('LoginFailed', () => {
    return <LoginFailed />;
  });
