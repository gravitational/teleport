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
import cfg from 'teleport/config';
import PasswordForm from 'shared/components/FormPassword';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import authService from 'teleport/services/auth';

export function Account(props) {
  const { auth2faType, onChangePass, onChangePassWithU2f } = props;
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Account Settings</FeatureHeaderTitle>
      </FeatureHeader>
      <PasswordForm
        auth2faType={auth2faType}
        onChangePass={onChangePass}
        onChangePassWithU2f={onChangePassWithU2f}
      />
    </FeatureBox>
  );
}

export default function(props) {
  const settProps = {
    ...props,
    auth2faType: cfg.getAuth2faType(),
    onChangePass: authService.changePassword.bind(authService),
    onChangePassWithU2f: authService.changePasswordWithU2f.bind(authService),
  };

  return <Account {...settProps} />;
}
