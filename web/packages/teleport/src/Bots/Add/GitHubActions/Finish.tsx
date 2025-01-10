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

import { Link } from 'react-router-dom';

import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import Image from 'design/Image';
import Text, { H2 } from 'design/Text';

import cfg from 'teleport/config';
import celebratePamPng from 'teleport/Discover/Shared/Finished/celebrate-pam.png';

import { useGitHubFlow } from './useGitHubFlow';

export function Finish() {
  const { createBotRequest } = useGitHubFlow();

  return (
    <Flex
      width="600px"
      flexDirection="column"
      alignItems="center"
      mt={5}
      css={`
        margin-right: auto;
        margin-left: auto;
        text-align: center;
      `}
    >
      <Image width="120px" height="120px" src={celebratePamPng} />
      <H2 mt={3} mb={2}>
        Your Bot is Added to Teleport
      </H2>
      <Text mb={3}>
        Bot {createBotRequest.botName} has been successfully added to this
        Teleport Cluster. You can see {createBotRequest.botName} in the Bots
        page and you can always find the sample GitHub Actions workflow again
        from the bot's options.
      </Text>
      <Flex gap="4">
        <ButtonPrimary as={Link} to={cfg.getBotsRoute()} size="large">
          View Bots
        </ButtonPrimary>
        <ButtonSecondary as={Link} to={cfg.getBotsNewRoute()} size="large">
          Add Another Bot
        </ButtonSecondary>
      </Flex>
    </Flex>
  );
}
