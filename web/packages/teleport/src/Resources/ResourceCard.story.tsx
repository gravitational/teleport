import React from 'react';
import { Meta, StoryObj } from '@storybook/react';
import { ResourceCard as ResourceCard } from './ResourceCard';
import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';
import Flex from 'design/Flex';
import Box from 'design/Box';
import styled from 'styled-components';
import { gap } from 'design/system';
import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

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
