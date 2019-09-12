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
import { Input } from 'design';
import cfg from 'teleport/config';

export default function FieldInputSsh() {
  const [hasError, setHasError] = React.useState(false);
  function onKeyPress(e) {
    const value = e.target.value;
    if ((e.key === 'Enter' || e.type === 'click') && value) {
      const valid = check(value);
      setHasError(!valid);
      if (valid) {
        const [login, host] = value.split('@');
        const route = cfg.getConsoleConnectRoute({
          serverId: host,
          login,
        });

        window.open(route);
      }
    } else {
      setHasError(false);
    }
  }

  return (
    <Input
      hasError={hasError}
      height="34px"
      width="200px"
      bg="primary.dark"
      color="text.primary"
      placeholder="login@host"
      onKeyPress={onKeyPress}
    />
  );
}

const SSH_STR_REGEX = /(^\w+@(\S+)$)/;
const check = value => {
  const match = SSH_STR_REGEX.exec(value);
  return match !== null;
};
