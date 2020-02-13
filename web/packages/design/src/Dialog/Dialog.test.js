/**
 * Copyright 2020 Gravitational, Inc.
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
import Dialog from './Dialog';
import DialogContent from './DialogContent';
import { render } from 'design/utils/testing';

const testCss = {
  'background-color': '#fff',
  color: '#000',
};

describe('design/Dialog', () => {
  it('respects dialogCss prop', () => {
    const { getByTestId } = render(
      <Dialog open={true} dialogCss={() => testCss}>
        <div>hello</div>
      </Dialog>
    );

    expect(getByTestId('dialogbox')).toHaveStyle({ ...testCss });
  });

  it('renders content', () => {
    const { getByTestId } = render(
      <Dialog open={true}>
        <DialogContent data-testid="test">hello</DialogContent>
      </Dialog>
    );

    expect(getByTestId('test')).not.toBeNull();
  });
});
