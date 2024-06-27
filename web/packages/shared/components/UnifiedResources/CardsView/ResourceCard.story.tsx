/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';
import { Meta, StoryObj } from '@storybook/react';

import styled from 'styled-components';

import { ButtonBorder } from 'design';
import { gap, GapProps } from 'design/system';

import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';

import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

import makeApp from 'teleport/services/apps/makeApps';
import { ResourceActionButton } from 'teleport/UnifiedResources/ResourceActionButton';

import {
  makeUnifiedResourceViewItemApp,
  makeUnifiedResourceViewItemDatabase,
  makeUnifiedResourceViewItemKube,
  makeUnifiedResourceViewItemNode,
  makeUnifiedResourceViewItemDesktop,
} from '../shared/viewItemsFactory';

import { PinningSupport } from '../types';

import { ResourceCard } from './ResourceCard';

const additionalResources = [
  makeApp({
    name: 'An application with an awfully long name that will be truncated',
    uri: 'https://you.should.be.ashamed.of.yourself.for.picking.such.ungodly.long.domain.names/',
    description: 'I love the smell of word wrapping in the morning.',
    awsConsole: false,
    labels: [
      {
        name: 'some-rather-long-label-name',
        value:
          "I don't like to be labeled, do you? I find labels opressive. Label " +
          "me, and I'll label you back. Or at least truncate my one.",
      },
    ],
    clusterId: 'one',
    fqdn: 'jenkins.one',
  }),
  makeApp({
    name: 'An application with a lot of labels',
    uri: 'http://localhost/',
    labels: [
      { name: 'day1', value: 'a partridge in a pear tree' },
      { name: 'day2', value: 'two turtle doves' },
      { name: 'day3', value: 'three French hens' },
      { name: 'day4', value: 'four calling birds' },
      { name: 'day5', value: 'five gold rings' },
      { name: 'day6', value: 'six geese a-laying' },
      { name: 'day7', value: 'seven swans a-swimming' },
      { name: 'day8', value: 'eight maids a-milking' },
      { name: 'day9', value: 'nine ladies dancing' },
      { name: 'day10', value: 'ten lords a-leaping' },
      { name: 'day11', value: 'eleven pipers piping' },
      { name: 'day12', value: 'twelve drummers drumming' },
    ],
  }),
];

const meta: Meta<typeof ResourceCard> = {
  component: ResourceCard,
  title: 'Shared/UnifiedResources/Items',
};

const Grid = styled.div<GapProps>`
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
  ${gap}
`;

export default meta;
type Story = StoryObj<typeof ResourceCard>;

const ActionButton = <ButtonBorder size="small">Action</ButtonBorder>;

export const Cards: Story = {
  render() {
    return (
      <Grid gap={2}>
        {[
          ...apps.map(resource =>
            makeUnifiedResourceViewItemApp(resource, {
              ActionButton: <ResourceActionButton resource={resource} />,
            })
          ),
          ...databases.map(resource =>
            makeUnifiedResourceViewItemDatabase(resource, {
              ActionButton,
            })
          ),
          ...kubes.map(resource =>
            makeUnifiedResourceViewItemKube(resource, { ActionButton })
          ),
          ...nodes.map(resource =>
            makeUnifiedResourceViewItemNode(resource, {
              ActionButton,
            })
          ),
          ...additionalResources.map(resource =>
            makeUnifiedResourceViewItemApp(resource, { ActionButton })
          ),
          ...desktops.map(resource =>
            makeUnifiedResourceViewItemDesktop(resource, { ActionButton })
          ),
        ].map((res, i) => (
          <ResourceCard
            key={i}
            pinned={false}
            pinResource={() => {}}
            selectResource={() => {}}
            selected={false}
            pinningSupport={PinningSupport.Supported}
            name={res.name}
            primaryIconName={res.primaryIconName}
            SecondaryIcon={res.SecondaryIcon}
            cardViewProps={res.cardViewProps}
            labels={res.labels}
            ActionButton={res.ActionButton}
          />
        ))}
      </Grid>
    );
  },
};
