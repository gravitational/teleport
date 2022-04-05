/*
Copyright 2021-2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import Welcome from './Welcome';
import { NewCredentials, Props } from './NewCredentials';
import { mockedProps } from './fixtures/fixtures';

export default { title: 'Teleport/Welcome' };

export const GetStarted = () => <MockedWelcome url="/web/invite/1234" />;

export const Continue = () => (
  <MockedWelcome url="/web/invite/1234/continue" Form={Otp} />
);

export const GetStartedReset = () => <MockedWelcome url="/web/reset/1234" />;

export const ContinueReset = () => (
  <MockedWelcome url="/web/reset/1234/continue" Form={Otp} />
);

export const AuthMfaOn = () => (
  <MockedWelcome url="/web/invite/1234/continue" Form={MfaOn} />
);

export const AuthMfaOptional = () => (
  <MockedWelcome url="/web/invite/1234/continue" Form={MfaOptional} />
);

export const AuthMfaOnOtp = () => (
  <MockedWelcome url="/web/invite/1234/continue" Form={Otp} />
);

const MfaOn = (props: Props) => (
  <NewCredentials {...props} {...mockedProps} auth2faType="on" />
);

const Otp = (props: Props) => (
  <NewCredentials {...props} {...mockedProps} auth2faType="otp" />
);

const MfaOptional = (props: Props) => (
  <NewCredentials {...props} {...mockedProps} auth2faType="optional" />
);

type MockedWelcomeProps = {
  url: string;
  Form?: React.FC<Props>;
};

function MockedWelcome({ url, Form }: MockedWelcomeProps) {
  return (
    <MemoryRouter initialEntries={[url]}>
      <Welcome CustomForm={Form} />
    </MemoryRouter>
  );
}

GetStarted.storyName = 'GetStarted';
GetStartedReset.storyName = 'GetStartedReset';
ContinueReset.storyName = 'ContinueReset';
Continue.storyName = 'Continue';
AuthMfaOnOtp.storyName = 'MfaOnOtp';
AuthMfaOn.storyName = 'MfaOn';
AuthMfaOptional.storyName = 'MfaOptional';
