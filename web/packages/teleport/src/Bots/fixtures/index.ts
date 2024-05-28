/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { ApiBot, BotResponse, FlatBot } from 'teleport/services/bot/types';

// nonDisplayedFields are not leveraged in the UI, so we don't explicitly set them
const nonDisplayedFields = {
  namespace: '',
  description: '',
  labels: null,
  revision: '',
  traits: [],
  status: '',
  subKind: '',
  version: '',
};

export const botsFixture: FlatBot[] = [
  {
    ...nonDisplayedFields,
    kind: 'GitHub Actions',
    name: 'bot-github-actions',
    roles: ['bot-bot-role'],
  },
  {
    ...nonDisplayedFields,
    kind: 'IAM',
    name: 'bot-slack-iam',
    roles: ['bot-iam-role'],
  },
  {
    ...nonDisplayedFields,
    kind: 'GitHub SSO',
    name: 'github-integration',
    roles: [],
  },
  {
    ...nonDisplayedFields,
    kind: 'Access Plugin',
    name: 'Pagerduty',
    roles: ['access-plugin'],
  },
  {
    ...nonDisplayedFields,
    kind: 'Terraform',
    name: 'terraform',
    roles: [],
  },
];

const getEmptyApiBot = (
  kind: string,
  name: string,
  roles: string[]
): ApiBot => ({
  kind: kind,
  metadata: {
    description: '',
    labels: null,
    name: name,
    namespace: '',
    revision: '',
  },
  spec: {
    roles: roles,
    traits: [],
  },
  status: '',
  subKind: '',
  version: '',
});

export const botsApiResponseFixture: BotResponse = {
  items: [
    {
      ...getEmptyApiBot('GitHub Actions', 'bot-github-actions', [
        'bot-bot-role',
      ]),
    },
    {
      ...getEmptyApiBot('IAM', 'bot-slack-iam', ['bot-iam-role']),
    },
    {
      ...getEmptyApiBot('GitHub SSO', 'github-integration', []),
    },
    {
      ...getEmptyApiBot('Access Plugin', 'Pagerduty', ['access-plugin']),
    },
    {
      ...getEmptyApiBot('Terraform', 'terraform', []),
    },
  ],
};
