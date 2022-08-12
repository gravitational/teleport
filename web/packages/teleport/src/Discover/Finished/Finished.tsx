/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { ButtonPrimary, Text, Flex, ButtonText, Image } from 'design';

import cfg from 'teleport/config';
import history from 'teleport/services/history';

import celebratePamPng from './celebrate-pam.png';

export function Finished() {
  return (
    <Flex
      width="400px"
      flexDirection="column"
      alignItems="center"
      css={`
        margin: 0 auto;
      `}
    >
      <Image width="120px" height="120px" src={celebratePamPng} />
      <Text my={3} typography="h4" bold>
        Resource Successfully Connected
      </Text>
      <ButtonPrimary
        width="100%"
        size="large"
        onClick={() => history.push(cfg.routes.root, true)}
      >
        Go to Access Provider
      </ButtonPrimary>
      <ButtonText
        pt={2}
        width="100%"
        size="large"
        onClick={() => history.reload()}
      >
        Add Another Resource
      </ButtonText>
    </Flex>
  );
}
