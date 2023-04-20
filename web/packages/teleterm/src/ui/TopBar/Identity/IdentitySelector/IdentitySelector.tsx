/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { forwardRef } from 'react';
import { Box } from 'design';

import { getUserWithClusterName } from 'teleterm/ui/utils';

import { TopBarButton } from 'teleterm/ui/TopBar/TopBarButton';

import { UserIcon } from './UserIcon';
import { PamIcon } from './PamIcon';

interface IdentitySelectorProps {
  isOpened: boolean;
  userName: string;
  clusterName: string;

  onClick(): void;
  makeTitle: (userWithClusterName: string | undefined) => string;
}

export const IdentitySelector = forwardRef<
  HTMLButtonElement,
  IdentitySelectorProps
>((props, ref) => {
  const isSelected = props.userName && props.clusterName;
  const selectorText = isSelected && getUserWithClusterName(props);
  const title = props.makeTitle(selectorText);

  return (
    <TopBarButton
      isOpened={props.isOpened}
      ref={ref}
      onClick={props.onClick}
      title={title}
    >
      {isSelected ? (
        <Box>
          <UserIcon letter={props.userName[0]} />
        </Box>
      ) : (
        <PamIcon />
      )}
    </TopBarButton>
  );
});
