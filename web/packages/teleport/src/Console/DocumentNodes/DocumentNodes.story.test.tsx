/*
Copyright 2019-2020 Gravitational, Inc.

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
import { Document, createContext } from './DocumentNodes.story';
import { wait, render } from 'design/utils/testing';

test('render DocumentNodes', async () => {
  const ctx = createContext();
  jest.spyOn(ctx, 'fetchClusters');
  jest.spyOn(ctx, 'fetchNodes');

  const { container } = render(<Document value={ctx} />);
  await wait(() => expect(ctx.fetchClusters).toHaveBeenCalledTimes(1));
  await wait(() => expect(ctx.fetchNodes).toHaveBeenCalledTimes(1));
  expect(container.firstChild).toMatchSnapshot();
});
