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

import { Record, Map } from 'immutable';
import { isString } from 'lodash';

export class ItemRec extends Record({
  isNew: false,
  id: '',
  kind: '',
  name: '',
  displayName: '',
  content: ''
}){
  constructor(props={}){
    super({
      displayName: props.name,
      ...props,
    })
  }

  getId(){
    return this.get('id');
  }

  getIsNew(){
    return this.get('isNew');
  }

  getName(){
    return this.get('name');
  }

  getContent(){
    return this.get('content');
  }

  getKind(){
    return this.get('kind');
  }
}

export class StoreRec extends Record({
  items: Map()
}){

  getItems(){
    return this.items.valueSeq().sortBy(i => i.getName());
  }

  upsertItems(jsonItems){
    let itemMap = this.get('items');
    jsonItems.forEach(json => {
      const rec = new ItemRec(json);
      itemMap = itemMap.set(rec.id, rec)
    })

    return this.set('items', itemMap);
  }

  findItem(item /* string|itemRec*/){
    if(!item) {
      return null;
    }

    const id = isString(item) ? item : item.id;
    return this.getIn(['items', id]);
  }

  removeAll(){
    return this.set('items', new Map())
  }

  remove(id){
    return this.deleteIn(['items', id]);
  }

  createItem(){
    return new ItemRec({ isNew: true });
  }

  setItems(jsonItems){
    let store = this.removeAll();
    return store.upsertItems(jsonItems);
  }
}