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

import Box from 'design/Box';
import { Mark } from 'design/Mark';

import {
  InfoExternalTextLink,
  InfoParagraph,
  ReferenceLinks,
} from 'teleport/components/SlidingSidePanel/InfoGuideSidePanel';

const InfoGuideReferenceLinks = {
  Bots: {
    title: 'What are Bots',
    href: 'https://goteleport.com/docs/enroll-resources/machine-id/introduction/#bots',
  },
  TBot: {
    title: 'What is tbot',
    href: 'https://goteleport.com/docs/reference/architecture/machine-id-architecture/#tbot',
  },
  AccessResources: {
    title: 'Access your Infrastructure with Machine ID',
    href: 'https://goteleport.com/docs/enroll-resources/machine-id/access-guides/',
  },
};

export const InfoGuide = () => (
  <Box>
    <InfoParagraph>
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.Bots.href}
      >
        Bots
      </InfoExternalTextLink>{' '}
      are identities a machine can use to authenticate to the Teleport cluster.
      This allows processes like automated tests,
      Infrastructure-as-Code/provisioning tools like Terraform or Ansible and
      scripts to{' '}
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.AccessResources.href}
      >
        access resources
      </InfoExternalTextLink>{' '}
      protected by the Teleport proxy.
    </InfoParagraph>
    <InfoParagraph>
      Bots use the{' '}
      <InfoExternalTextLink
        target="_blank"
        href={InfoGuideReferenceLinks.TBot.href}
      >
        tbot
      </InfoExternalTextLink>{' '}
      binary rather than the <Mark>teleport</Mark> binary used for other agents.{' '}
      <Mark>tbot</Mark> outputs identity files such as certificates and
      Kubernetes configuration files for processes to use for authentication.
    </InfoParagraph>
    <ReferenceLinks links={Object.values(InfoGuideReferenceLinks)} />
  </Box>
);
