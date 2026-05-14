// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import { ButtonSecondary, ResourceIcon, Stack, Text } from 'design';
import { NewTab } from 'design/Icon';

import cfg from 'e-teleport/config';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout/Layout';
import { useNoMinWidth } from 'teleport/Main';

import { BeamsCard } from './components';

const PAGE_MAX_WIDTH = 800;
const SLACK_ICON_WIDTH = '56px';

export function BeamsFeedback() {
  useNoMinWidth();

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>Feedback</FeatureHeaderTitle>
      </FeatureHeader>
      <BeamsCard maxWidth={PAGE_MAX_WIDTH} mx="auto" width="100%">
        <Stack alignItems="center" gap={4}>
          <ResourceIcon name="slack" width={SLACK_ICON_WIDTH} />
          <Text typography="body1" textAlign="center">
            Share feedback at <strong>#beams</strong> in the Teleport Community
            Slack.
          </Text>
          <ButtonSecondary
            as="a"
            href={cfg.communitySlackUrl}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Join Teleport on Slack (opens in a new tab)"
            gap={2}
          >
            Join Teleport on Slack
            <NewTab size="small" />
          </ButtonSecondary>
        </Stack>
      </BeamsCard>
    </FeatureBox>
  );
}
