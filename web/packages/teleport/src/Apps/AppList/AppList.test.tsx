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
import { render, screen, fireEvent } from 'design/utils/testing';

import cfg from 'teleport/config';

import { props } from '../Apps.story';

import AppList from './AppList';

test('correct launch url is generated for a selected role', () => {
  jest.spyOn(cfg, 'getAppLauncherRoute');

  render(
    <AppList
      {...props}
      apps={[
        {
          kind: 'app',
          name: 'aws-console-1',
          uri: 'https://console.aws.amazon.com/ec2/v2/home',
          publicAddr: 'awsconsole-1.teleport-proxy.com',
          labels: [
            {
              name: 'aws_account_id',
              value: 'A1234',
            },
            {
              name: 'env',
              value: 'dev',
            },
            {
              name: 'cluster',
              value: 'two',
            },
          ],
          description: 'This is an AWS Console app',
          awsConsole: true,
          awsRoles: [
            {
              arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
              display: 'EC2FullAccess',
            },
            {
              arn: 'arn:aws:iam::joe123:role/EC2ReadOnly',
              display: 'EC2ReadOnly',
            },
          ],
          clusterId: 'one',
          fqdn: 'awsconsole-1.com',
          id: 'one-aws-console-1-awsconsole-1.teleport-proxy.com',
          launchUrl: '',
          userGroups: [],
          samlApp: false,
          samlAppSsoUrl: '',
        },
      ]}
    />
  );

  const launchBtn = screen.queryByText(/launch/i);

  fireEvent.click(launchBtn);

  expect(cfg.getAppLauncherRoute).toHaveBeenCalledWith({
    fqdn: 'awsconsole-1.com',
    clusterId: 'one',
    publicAddr: 'awsconsole-1.teleport-proxy.com',
    arn: 'arn:aws:iam::joe123:role/EC2FullAccess',
  });

  expect(cfg.getAppLauncherRoute).toHaveBeenCalledWith({
    fqdn: 'awsconsole-1.com',
    clusterId: 'one',
    publicAddr: 'awsconsole-1.teleport-proxy.com',
    arn: 'arn:aws:iam::joe123:role/EC2ReadOnly',
  });

  const launchUrl = screen
    .queryByText('EC2ReadOnly')
    .closest('a')
    .getAttribute('href');

  expect(launchUrl).toBe(
    '/web/launch/awsconsole-1.com/one/awsconsole-1.teleport-proxy.com/arn:aws:iam::joe123:role%2FEC2ReadOnly'
  );
});
