/*
Copyright 2020 Gravitational, Inc.

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
import { render } from 'design/utils/testing';
import * as story from './FormInvite.story';

describe('shared/components/Invite.story', () => {
  test('story.off', () => {
    const { container } = render(<story.Off />);
    expect(container.firstChild).toMatchSnapshot();
  });

  test('story.Otp', () => {
    const { container } = render(<story.Otp />);
    expect(container.firstChild).toMatchSnapshot();
  });

  test('story.U2f', () => {
    const { container } = render(<story.U2f />);
    expect(container.firstChild).toMatchSnapshot();
  });

  test('story.OtpError', () => {
    const { container } = render(<story.OtpError />);
    expect(container.firstChild).toMatchSnapshot();
  });
});
