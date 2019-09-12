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

import { Store, useStore } from 'gravity/lib/stores';
import * as resources from 'gravity/services/resources';
import { ResourceEnum } from 'gravity/services/enums';

export const ModeEnum = {
  VIEW: 'view',
  EDIT: 'edit',
  NEW: 'new'
}

export class LogforwarderStore extends Store {

  state = {
    siteId: null,
    isNew: false,
    mode: ModeEnum.VIEW,
    curIndex: 0,
    items: []
  }

  setCurrent = curIndex => {
    this.setState({ curIndex });
  }

  setViewMode() {
    this.setState({ mode: ModeEnum.VIEW});
  }

  setNewMode(){
    this.setState({ mode: ModeEnum.NEW});
  }

  setEditMode(){
    this.setState({ mode: ModeEnum.EDIT});
  }

  setItems(items){
    this.setState({ items });
  }

  fetch(){
    return resources.getForwarders()
      .then(items => this.setState({ curIndex: 0, items}));
  }

  save(content){
    const { items, mode } = this.state;
    const isNew = mode === ModeEnum.NEW;
    return resources.upsert(ResourceEnum.LOG_FWRD, content, isNew)
      .done(inserted => {
        let { curIndex } = this.state;
        if(!isNew){
          items[curIndex] = inserted[0];
        }else{
          items.push(inserted[0]);
          curIndex = items.length - 1;
        }

        this.setState({
          items: [...items],
          mode: ModeEnum.VIEW,
          curIndex
        })
      })
  }

  delete = index => {
    const { items } = this.state;
    const { name } = items[index];
    return resources.remove(ResourceEnum.LOG_FWRD, name)
      .then(() => {
        items.splice(index, 1);
        this.setState({
          items: [...items],
          mode: ModeEnum.VIEW,
          curIndex: items.length -1 < index ? index -1 : index,
        })
      })
  }
}

export {
  useStore
}