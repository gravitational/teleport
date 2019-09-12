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

import React from 'react'
import { storiesOf } from '@storybook/react'
import { useAttempt } from 'shared/hooks';
import { LogForwarderDialog } from './LogForwarderDialog'
import { LogforwarderStore } from './store';
import {  useStore } from 'gravity/lib/stores';

storiesOf('Gravity/Logs', module)
  .add('Forwarders Empty', () => {
    return (
      <Forwarders store={ new LogforwarderStore() } />
    )
  })
  .add('Forwarders View Mode', () => {
    const store = new LogforwarderStore();
    store.setItems(json)
    return (
      <Forwarders store={store} />
    );
  })
  .add('Forwarders Edit Mode', () => {
    const store = new LogforwarderStore();
    store.setItems(json);
    store.setEditMode();
    return (
      <Forwarders store={store} />
    );
  });

  function Forwarders(props) {
    const store = useStore(props.store);
    const [ attempt, attemptActions ] = useAttempt();
    return (
      <LogForwarderDialog {...props} store={store} attempt={attempt} attemptActions={attemptActions}/>
    )
  }

  const json = [
    { id: '189', name: 'Lewholtim', content: "Greece Obekidmu 247" },
    { id: '119', name: 'Hubepaza', content: "Spain Zejjihbuw 200" },
    { id: '203', name: 'Gugvaje', content: "Syria Nepabapu 243" },
    { id: '194', name: 'Foemipev', content: "Norfolk Island Upuniru 220" },
    { id: '211', name: 'Meudol', content: "Aruba Womohut 138" },
    { id: '195', name: 'Nouhorob', content: "Malawi Rombafza 159" },
    { id: '210', name: 'Meudol', content: "Aruba Womohut 138" },
    { id: '177', name: 'Nouhorob', content: "Malawi Rombafza 159" },
    { id: '10', name: 'Meudol', content: "Aruba Womohut 138" },
    { id: '154', name: 'Nouhorob', content: "Malawi Rombafza 159" },
  ];