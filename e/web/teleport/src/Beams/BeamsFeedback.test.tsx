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

import { render, screen } from 'design/utils/testing';
import { InfoGuidePanelProvider } from 'shared/components/SlidingSidePanel/InfoGuide';

import cfg from 'e-teleport/config';
import { ContentMinWidth } from 'teleport/Main/Main';

import { BeamsFeedback } from './BeamsFeedback';

test('renders a Slack link with the community URL', () => {
  render(
    <InfoGuidePanelProvider>
      <ContentMinWidth>
        <BeamsFeedback />
      </ContentMinWidth>
    </InfoGuidePanelProvider>
  );

  const slackLink = screen.getByRole('link', {
    name: /Join Teleport on Slack/i,
  });
  expect(slackLink).toHaveAttribute('href', cfg.communitySlackUrl);
  expect(slackLink).toHaveAttribute('target', '_blank');
  expect(slackLink).toHaveAttribute('rel', 'noopener noreferrer');

  expect(screen.getByTestId('res-icon-slack')).toBeInTheDocument();
});
