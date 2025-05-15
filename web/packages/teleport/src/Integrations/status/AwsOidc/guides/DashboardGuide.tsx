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

const DashboardGuideReferenceLinks = {
  AwsOidcIntegration: {
    title: 'AWS OIDC Integration',
    href: 'https://goteleport.com/docs/admin-guides/management/guides/awsoidc-integration/',
  },
};

export const DashboardGuide = () => (
  <Box>
    <InfoParagraph>
      This Integration allows you to access protected AWS resources from
      Teleport. It uses AWS IAM OIDC Identity Provider to access AWS APIs. You
      can read more about how the integration works{' '}
      <InfoExternalTextLink
        target="_blank"
        href={DashboardGuideReferenceLinks.AwsOidcIntegration.href}
      >
        here
      </InfoExternalTextLink>
    </InfoParagraph>
    <InfoParagraph>
      Teleport detects resources in your AWS Account and enrolls them in your
      Teleport cluster. When you deploy servers, databases, and Kubernetes
      clusters, Teleport enables secure access to these resources with no
      further configuration. This lets you decouple the need to protect your
      infrastructure resources from the work of deploying and managing them.
    </InfoParagraph>
    <InfoParagraph>
      You can configure which resources to enroll by creating enrollment rules.
    </InfoParagraph>
    <InfoParagraph>
      This page has a summary of the current enrollment rules, and onboarded
      resources. Click on each resource type to get more details.
    </InfoParagraph>
    <ReferenceLinks links={Object.values(DashboardGuideReferenceLinks)} />
  </Box>
);
