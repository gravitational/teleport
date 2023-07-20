/**
 * Copyright 2021 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
