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

import { ButtonBorder, Flex } from 'design';

import { apps } from 'teleport/Apps/fixtures';
import { databases } from 'teleport/Databases/fixtures';

import { kubes } from 'teleport/Kubes/fixtures';
import { desktops } from 'teleport/Desktops/fixtures';
import { nodes } from 'teleport/Nodes/fixtures';

import makeApp from 'teleport/services/apps/makeApps';

import {
  makeUnifiedResourceViewItemApp,
  makeUnifiedResourceViewItemDatabase,
  makeUnifiedResourceViewItemKube,
  makeUnifiedResourceViewItemNode,
  makeUnifiedResourceViewItemDesktop,
} from '../shared/viewItemsFactory';

import { PinningSupport } from '../types';

import { ResourceListItem } from './ResourceListItem';

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

const meta: Meta<typeof ResourceListItem> = {
  component: ResourceListItem,
  title: 'Shared/UnifiedResources/Items',
};

export default meta;
type Story = StoryObj<typeof ResourceListItem>;

const ActionButton = <ButtonBorder size="small">Action</ButtonBorder>;

export const ListItems: Story = {
  render() {
    return (
      <Flex flexDirection="column">
        {[
          ...apps.map(resource =>
            makeUnifiedResourceViewItemApp(resource, { ActionButton })
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
            makeUnifiedResourceViewItemNode(resource, { ActionButton })
          ),
          ...additionalResources.map(resource =>
            makeUnifiedResourceViewItemApp(resource, { ActionButton })
          ),
          ...desktops.map(resource =>
            makeUnifiedResourceViewItemDesktop(resource, { ActionButton })
          ),
        ].map((res, i) => (
          <ResourceListItem
            key={i}
            pinned={false}
            pinResource={() => {}}
            selectResource={() => {}}
            selected={false}
            pinningSupport={PinningSupport.Supported}
            name={res.name}
            primaryIconName={res.primaryIconName}
            SecondaryIcon={res.SecondaryIcon}
            description={res.description}
            type={res.type}
            addr={res.addr}
            labels={res.labels}
            ActionButton={res.ActionButton}
          />
        ))}
      </Flex>
    );
  },
};
