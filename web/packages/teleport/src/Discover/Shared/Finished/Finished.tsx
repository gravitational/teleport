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

import styled from 'styled-components';

import { ButtonPrimary, ButtonSecondary, Flex, H2, Image, Text } from 'design';

import cfg from 'teleport/config';
import history from 'teleport/services/history';

import type { AgentStepProps } from '../../types';
import celebratePamPng from './celebrate-pam.png';

export function Finished(props: AgentStepProps) {
  let title = 'Resource Successfully Added';
  let resourceText =
    'You can start accessing this resource right away or add another resource.';

  if (props.agentMeta) {
    if (props.agentMeta.autoDiscovery) {
      title = 'Completed Setup';
      resourceText = 'You have completed setup for auto-enrolling.';
    } else if (props.agentMeta.resourceName) {
      resourceText = `Resource [${props.agentMeta.resourceName}] has been successfully added to
      this Teleport Cluster. ${resourceText}`;
    }
  }

  return (
    <Container>
      <Image width="120px" height="120px" src={celebratePamPng} />
      <H2 mt={3} mb={2}>
        {title}
      </H2>
      <Text mb={3}>{resourceText}</Text>
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
    </Container>
  );
}

const Container = styled(Flex)`
  width: 600px;
  flex-direction: column;
  align-items: center;
  margin: 0 auto;
  text-align: center;
`;
