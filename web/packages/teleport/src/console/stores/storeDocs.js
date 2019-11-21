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

import { Store } from 'shared/libs/stores';
import { SessionStateEnum } from 'teleport/services/termsessions';

export default class StoreDocs extends Store {
  state = {
    items: [],
    active: -1,
  };

  add(json, makeActive = false) {
    const newItem = {
      id: Math.floor(Math.random() * 100000),
      ...json,
    };

    const active = makeActive ? newItem.id : this.state.active;

    this.setState({
      items: [...this.state.items, newItem],
      active,
    });

    return newItem;
  }

  updateItem(id, json) {
    const items = this.state.items.map(item => {
      if (item.id === id) {
        return {
          ...item,
          ...json,
        };
      }

      return item;
    });

    this.setState({
      items,
    });
  }

  filter(id) {
    return this.state.items.filter(i => i.id !== id);
  }

  hasActiveTerminalSessions() {
    return this.state.items.some(i => i.status === SessionStateEnum.CONNECTED);
  }

  getNext(id) {
    const { items } = this.state;
    for (let i = 0; i < items.length; i++) {
      if (items[i].id === id) {
        if (items.length > i + 1) {
          return items[i + 1].id;
        }

        if (items.length === i + 1 && i !== 0) {
          return items[i - 1].id;
        }
      }
    }

    return -1;
  }

  find(id) {
    return this.state.items.find(i => i.id === id);
  }

  getItem(id) {
    return this.state.find(i => i.id === id);
  }
}
