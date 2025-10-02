/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import Box from 'design/Box/Box';
import { Mark } from 'design/Mark/Mark';
import {
  InfoExternalTextLink,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide/InfoGuide';

export function InfoGuide() {
  return (
    <Box>
      <InfoParagraph>
        A{' '}
        <InfoExternalTextLink
          target="_blank"
          href={InfoGuideReferenceLinks.BotInstances.href}
        >
          Bot Instance
        </InfoExternalTextLink>{' '}
        identifies a single lineage of{' '}
        <InfoExternalTextLink
          target="_blank"
          href={InfoGuideReferenceLinks.Bots.href}
        >
          bot
        </InfoExternalTextLink>{' '}
        identities, even through certificate renewals and rejoins. When the{' '}
        <Mark>tbot</Mark> client first authenticates to a cluster, a Bot
        Instance is generated and its UUID is embedded in the returned client
        identity.
      </InfoParagraph>
      <InfoParagraph>
        Bot Instances track a variety of information about <Mark>tbot</Mark>{' '}
        instances, including regular heartbeats which include basic information
        about the <Mark>tbot</Mark> host, like its architecture and OS version.
      </InfoParagraph>
      <InfoParagraph>
        Bot Instances have a relatively short lifespan and are set to expire
        after the most recent identity issued for that instance expires. If the{' '}
        <Mark>tbot</Mark> client associated with a particular Bot Instance
        renews or rejoins, the expiration of the bot instance is reset. This is
        designed to allow users to list Bot Instances for an accurate view of
        the number of active <Mark>tbot</Mark> clients interacting with their
        Teleport cluster.
      </InfoParagraph>
      <ReferenceLinks links={Object.values(InfoGuideReferenceLinks)} />
    </Box>
  );
}

const InfoGuideReferenceLinks = {
  BotInstances: {
    title: 'What are Bot instances',
    href: 'https://goteleport.com/docs/enroll-resources/machine-id/introduction/#bot-instances',
  },
  Bots: {
    title: 'What are Bots',
    href: 'https://goteleport.com/docs/enroll-resources/machine-id/introduction/#bots',
  },
  Tctl: {
    title: 'Use tctl to manage bot instances',
    href: 'https://goteleport.com/docs/reference/cli/tctl/#tctl-bots-instances-add',
  },
};
