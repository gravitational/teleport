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

/* eslint-disable @typescript-eslint/ban-ts-comment*/
import React from 'react';
import { Flex, Text } from 'design';
// @ts-ignore
import { AccessRequestCheckoutButton } from 'e-teleterm/ui/StatusBar/AccessRequestCheckoutButton';

import { useActiveDocumentClusterBreadcrumbs } from './useActiveDocumentClusterBreadcrumbs';
import { ShareFeedback } from './ShareFeedback';

export function StatusBar() {
  const clusterBreadcrumbs = useActiveDocumentClusterBreadcrumbs();

  return (
    <Flex
      width="100%"
      height="28px"
      css={`
        border-top: 1px solid ${props => props.theme.colors.spotBackground[1]};
      `}
      alignItems="center"
      justifyContent="space-between"
      px={2}
      overflow="hidden"
    >
      <Text
        color="text.slightlyMuted"
        fontSize="14px"
        css={`
          white-space: nowrap;
        `}
        title={clusterBreadcrumbs}
      >
        {clusterBreadcrumbs}
      </Text>
      <Flex gap={2} alignItems="center">
        <AccessRequestCheckoutButton />
        <ShareFeedback />
      </Flex>
    </Flex>
  );
}
