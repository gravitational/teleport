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

import { Box, Link } from 'design';
import {
  InfoParagraph,
  InfoTitle,
  ReferenceLinks,
} from 'shared/components/SlidingSidePanel/InfoGuide';

const GuideReferenceLinks = {
  AwsRolesAnywhere: {
    title: 'AWS Roles Anywhere',
    href: 'https://docs.aws.amazon.com/rolesanywhere/latest/userguide/introduction.html',
  },
};

export const Guide = ({ resourcesRoute }: { resourcesRoute: string }) => (
  <Box>
    <InfoTitle>How to Access Profiles as Resources</InfoTitle>
    <InfoParagraph>
      Teleport will periodically sync Roles Anywhere Profiles as AWS Access
      applications. You can import all Profiles, or use a filter to only import
      a subset of Profiles. You can access AWS by going to the{' '}
      <Link href={resourcesRoute} target="_blank">
        Resources page
      </Link>{' '}
      and select the Profile and IAM Role that you want to use. Other users can
      use the Profile and IAM Role by requesting access to it.
    </InfoParagraph>
    <ReferenceLinks links={Object.values(GuideReferenceLinks)} />
  </Box>
);
