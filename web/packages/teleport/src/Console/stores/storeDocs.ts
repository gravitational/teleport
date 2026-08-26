/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { Store } from 'shared/libs/stores';

import { Document, DocumentSsh } from './types';

interface State {
  items: Document[];
}

export default class StoreDocs extends Store<State> {
  state: State = {
    items: [],
  };

  add(doc: Document) {
    const item: Document = {
      id: Math.floor(Math.random() * 100000),
      ...doc,
    };

    this.setState({
      items: [...this.state.items, item],
    });

    return item;
  }

  update(id: number, partialDoc: Partial<Document>) {
    const items = this.state.items.map(doc => {
      if (doc.id === id) {
        return {
          ...doc,
          ...partialDoc,
        };
      }

      return doc;
    }) as Document[];

    this.setState({
      items,
    });
  }

  filter(id: number) {
    return this.state.items.filter(i => i.id !== id);
  }

  getNext(id: number) {
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

  find(id: number) {
    return this.state.items.find(i => i.id === id);
  }

  findByUrl(url: string) {
    return this.state.items.find(i => encodeURI(i.url.split('?')[0]) === url);
  }

  getNodeDocuments() {
    return this.state.items.filter(doc => doc.kind === 'nodes');
  }

  getSshDocuments() {
    return this.state.items.filter(
      doc => doc.kind === 'terminal' && doc.status === 'connected'
    ) as DocumentSsh[];
  }

  getDbDocuments() {
    return this.state.items.filter(doc => doc.kind === 'db');
  }

  getDocuments(): Document[] {
    return this.state.items;
  }
}
