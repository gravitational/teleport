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

import React from 'react';
import { ButtonPrimary, Text, Flex, ButtonSecondary, Image } from 'design';

import cfg from 'teleport/config';
import history from 'teleport/services/history';

import celebratePamPng from './celebrate-pam.png';

import type { AgentStepProps } from '../../types';

export function Finished(props: AgentStepProps) {
  let resourceText;
  if (props.agentMeta && props.agentMeta.resourceName) {
    resourceText = `Resource [${props.agentMeta.resourceName}] has been successfully added to
        this Teleport Cluster.`;
  }

  return (
    <Flex
      width="600px"
      flexDirection="column"
      alignItems="center"
      css={`
        margin: 0 auto;
        text-align: center;
      `}
    >
      <Image width="120px" height="120px" src={celebratePamPng} />
      <Text mt={3} mb={2} typography="h4" bold>
        Resource Successfully Added
      </Text>
      <Text mb={3}>
        {resourceText} You can start accessing this resource right away or add
        another resource.
      </Text>
      <Flex>
        <ButtonPrimary
          width="270px"
          size="large"
          onClick={() => history.push(cfg.routes.root, true)}
          mr={3}
        >
          Browse Existing Resources
        </ButtonPrimary>
        <ButtonSecondary
          width="270px"
          size="large"
          onClick={() => history.reload()}
        >
          Add Another Resource
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
