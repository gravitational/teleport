/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Flex, Text } from 'design';
import ButtonSso, { guessProviderType } from 'shared/components/ButtonSso';

import * as types from 'teleterm/ui/services/clusters/types';

const SSOBtnList = ({
  providers,
  prefixText,
  isDisabled,
  onClick,
  autoFocus = false,
}: Props) => {
  const $btns = providers.map((item, index) => {
    let { name, type, displayName } = item;
    const title = displayName || `${prefixText} ${name}`;
    const ssoType = guessProviderType(title, type as types.AuthProviderType);
    return (
      <ButtonSso
        key={index}
        title={title}
        ssoType={ssoType}
        disabled={isDisabled}
        autoFocus={index === 0 && autoFocus}
        onClick={e => {
          e.preventDefault();
          onClick(item);
        }}
      />
    );
  });

  if ($btns.length === 0) {
    return <Text typography="h3">You have no SSO providers configured</Text>;
  }

  return (
    <Flex flexDirection="column" gap={3}>
      {$btns}
    </Flex>
  );
};

type Props = {
  prefixText: string;
  isDisabled: boolean;
  onClick(provider: types.AuthProvider): void;
  providers: types.AuthProvider[];
  // autoFocus focuses on the first button in list.
  autoFocus?: boolean;
};

export default SSOBtnList;
