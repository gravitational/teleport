/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import type { Meta } from '@storybook/react';
import { MemoryRouter } from 'react-router';

import { Box, Flex } from 'design';
import {
  InfoGuideContainer,
  InfoGuidePanelProvider,
  useInfoGuide,
} from 'shared/components/SlidingSidePanel/InfoGuide';

import { CloudAws } from './CloudAws';

export const Flow = () => {
  const { infoGuideConfig, setInfoGuideConfig } = useInfoGuide();

  return (
    <MemoryRouter>
      <Flex height="100vh">
        <CloudAws />
        <Box
          width="380px"
          style={{ position: 'sticky', marginLeft: 20, top: 20 }}
        >
          {infoGuideConfig?.guide}
        </Box>
      </Flex>
    </MemoryRouter>
  );
};

const meta: Meta<typeof Flow> = {
  title: 'Teleport/Integrations/Enroll/CloudAws',
  component: Flow,
  decorators: [
    Story => (
      <InfoGuidePanelProvider>
        <Story />
      </InfoGuidePanelProvider>
    ),
  ],
};

export default meta;
