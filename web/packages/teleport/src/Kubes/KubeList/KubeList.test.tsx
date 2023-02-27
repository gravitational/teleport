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

import { kubes } from '../fixtures';

import { props } from '../Kubes.story';

import KubeList from './KubeList';

test('search generates correct url params', () => {
  const replaceHistory = jest.fn();

  render(
    <KubeList
      {...props}
      username="joe"
      authType="local"
      kubes={kubes}
      clusterId={'some-cluster-name'}
      pathname="test.com/cluster/one/kubes"
      replaceHistory={replaceHistory}
    />
  );

  fireEvent.change(screen.getByPlaceholderText(/SEARCH.../i), {
    target: { value: 'test' },
  });

  fireEvent.submit(screen.getByPlaceholderText(/SEARCH.../i));

  expect(replaceHistory).toHaveBeenCalledWith(
    'test.com/cluster/one/kubes?search=test&sort=name:asc'
  );
});
