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

import { Box, Mark } from 'design';
import {
  InfoExternalTextLink,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';

const EksGuideReferenceLinks = {
  KubeAgent: {
    title: 'teleport-kube-agent Chart Reference',
    href: 'https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/',
  },
};

export const EksGuide = () => (
  <Box>
    <InfoParagraph>
      Teleport scans AWS for EKS clusters that match specified region and
      filtering labels. For each discovered cluster, Teleport will install the
      <Mark>teleport-kube-agent</Mark> which joins your cluster. See how the
      Teleport Kubernetes Agent works{' '}
      <InfoExternalTextLink
        target="_blank"
        href={EksGuideReferenceLinks.KubeAgent.href}
      >
        here
      </InfoExternalTextLink>
    </InfoParagraph>
    <ReferenceLinks links={Object.values(EksGuideReferenceLinks)} />
  </Box>
);
