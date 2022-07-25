/**
 * Copyright 2022 Gravitational, Inc.
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

import Discover from './Discover';
import { DiscoverContext } from './discoverContext';
import DiscoverContextProvider from './discoverContextProvider';

// Main entry point to Discover where it initializes ContextProvider with the
// instance of DiscoverContext.
export default function Index() {
  const [ctx] = React.useState(() => {
    return new DiscoverContext();
  });

  return (
    <DiscoverContextProvider value={ctx}>
      <Discover />
    </DiscoverContextProvider>
  );
}
