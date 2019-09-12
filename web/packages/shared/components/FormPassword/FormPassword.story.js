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
import FormPassword from './FormPassword';
import { Auth2faTypeEnum } from '../../services/enums';

const onChangePass = () => Promise.resolve();
const onChangePassWithU2f = () => Promise.reject(new Error('server error'));

storiesOf('Shared/FormPassword', module)
  .add('FormPassword', () => {
    return (
      <FormPassword
        auth2faType
        onChangePass={onChangePass}
        onChangePassWithU2f={onChangePassWithU2f}
      />
    );
  })
  .add('With OTP', () => {
    return (
      <FormPassword
        auth2faType={Auth2faTypeEnum.OTP}
        onChangePass={onChangePass}
        onChangePassWithU2f={onChangePassWithU2f}
      />
    );
  })
  .add('With U2F', () => {
    return (
      <FormPassword
        auth2faType={Auth2faTypeEnum.UTF}
        onChangePass={onChangePass}
        onChangePassWithU2f={onChangePassWithU2f}
      />
    );
  });
