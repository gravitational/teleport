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

import React from 'react';

// This is the primary method to subscribe to store updates
// using React hooks mechanism.
export default function useStore(store){
  const [, rerender ] = React.useState();
  const memoizedState = React.useMemo(() => store.state, [store.state]);

  React.useEffect(() => {

    function syncState(){
      // do not re-render if state has not changed since call
      if(memoizedState !== store.state){
        rerender({});
      }
    }

    function onChange(){
      syncState();
    }

    // Sync state and force re-render if store has changed
    // during Component mount cycle
    syncState();

    // Subscribe to store changes
    store.subscribe(onChange);

    // Unsubscribe from store
    function cleanup(){
      store.unsubscribe(onChange)
    }

    return cleanup;

  }, [store]);

  return store;
}
