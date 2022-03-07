/*
Copyright 2019 Gravitational, Inc.

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

import { useAppContext } from 'teleterm/ui/appContextProvider';

export function useConnections() {
  const { connectionTracker } = useAppContext();

  connectionTracker.useState();

  const items = connectionTracker.getConnections();

  return {
    isAnyConnectionActive: items.some(c => c.connected),
    removeItem: (id: string) => connectionTracker.removeItem(id),
    activateItem: (id: string) => connectionTracker.activateItem(id),
    disconnectItem: (id: string) => connectionTracker.disconnectItem(id),
    items,
  };
}
