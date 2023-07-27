/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { Meta, StoryObj } from '@storybook/react';

import styled from 'styled-components';

import { gap } from 'design/system';

import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';

import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

import { ResourceCard as ResourceCard } from './ResourceCard';

const meta: Meta<typeof ResourceCard> = {
  component: ResourceCard,
  title: 'Teleport/Resources/ResourceCard',
};

const Grid = styled.div`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  ${gap}
`;

export default meta;
type Story = StoryObj<typeof ResourceCard>;

export const Cards: Story = {
  render() {
    return (
      <Grid gap={2}>
        {[...apps, ...databases, ...kubes, ...nodes, ...desktops].map(res => (
          <ResourceCard resource={res} />
        ))}
      </Grid>
    );
  },
};
