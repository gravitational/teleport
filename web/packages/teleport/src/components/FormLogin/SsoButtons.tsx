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

import React, { forwardRef } from 'react';
import { Box, Text } from 'design';
import ButtonSso, { guessProviderType } from 'shared/components/ButtonSso';
import { AuthProvider } from 'shared/services';

const SSOBtnList = forwardRef<HTMLInputElement, Props>(
  ({ providers, prefixText, isDisabled, onClick, autoFocus = false }, ref) => {
    const $btns = providers.map((item, index) => {
      let { name, type, displayName } = item;
      const title = displayName || `${prefixText} ${name}`;
      const ssoType = guessProviderType(title, type);
      const len = providers.length - 1;
      return (
        <ButtonSso
          setRef={index === 0 ? ref : null}
          key={index}
          title={title}
          ssoType={ssoType}
          disabled={isDisabled}
          mt={3}
          mb={index < len ? 3 : 0}
          autoFocus={index === 0 && autoFocus}
          onClick={e => {
            e.preventDefault();
            onClick(item);
          }}
        />
      );
    });

    if ($btns.length === 0) {
      return (
        <Text textAlign="center" bold pt={3}>
          You have no SSO providers configured
        </Text>
      );
    }

    return (
      <Box px={6} pt={2} pb={2} data-testid="sso-list">
        {$btns}
      </Box>
    );
  }
);

type Props = {
  prefixText: string;
  isDisabled: boolean;
  onClick(provider: AuthProvider): void;
  providers: AuthProvider[];
  // autoFocus focuses on the first button in list.
  autoFocus?: boolean;
};

export default SSOBtnList;
