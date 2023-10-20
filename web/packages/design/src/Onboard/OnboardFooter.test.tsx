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

import { render, screen } from 'design/utils/testing';
import { OnboardFooter } from 'design/Onboard/OnboardFooter';

test('renders RR, TOS, and PP', () => {
  render(<OnboardFooter />);

  expect(
    screen.getByText(/Gravitational, Inc. All Rights Reserved/i)
  ).toBeInTheDocument();
  expect(
    screen.getByRole('link', { name: /Terms of Service/i })
  ).toHaveAttribute('href', 'https://goteleport.com/legal/tos/');
  expect(screen.getByRole('link', { name: /Privacy Policy/i })).toHaveAttribute(
    'href',
    'https://goteleport.com/legal/privacy/'
  );
});
