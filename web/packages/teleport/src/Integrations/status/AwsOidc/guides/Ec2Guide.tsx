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

import { Box } from 'design';
import {
  InfoExternalTextLink,
  InfoParagraph,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';

const Ec2GuideReferenceLinks = {
  AwsIam: {
    title: 'Joining Services via AWS IAM Role',
    href: 'https://goteleport.com/docs/enroll-resources/agents/join-services-to-your-cluster/aws-iam/',
  },
};

export const Ec2Guide = () => (
  <Box>
    <InfoParagraph>
      Teleport can connect to Amazon EC2 and automatically discover and enroll
      EC2 instances matching the region and configured labels.
    </InfoParagraph>
    <InfoParagraph>
      It will then execute an install script on these discovered instances using
      AWS Systems Manager that will install Teleport, start it and join the
      cluster.
    </InfoParagraph>
    <InfoParagraph>
      In order to join the cluster, instances use an IAM invite token which
      allows only instances from this particular AWS Account to join your
      cluster.
    </InfoParagraph>
    <InfoParagraph>
      You can read more about how the IAM invite token works{' '}
      <InfoExternalTextLink
        target="_blank"
        href={Ec2GuideReferenceLinks.AwsIam.href}
      >
        here
      </InfoExternalTextLink>
    </InfoParagraph>
    <ReferenceLinks links={Object.values(Ec2GuideReferenceLinks)} />
  </Box>
);
