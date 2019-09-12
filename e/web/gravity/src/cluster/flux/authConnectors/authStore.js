import { Store } from 'nuclear-js';
import * as AT from './actionTypes';
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

  setContent(content){
    return this.set('content', content);
  }

  getContent(){
    return this.get('content');
  }

  getKind(){
    return this.get('kind');
  }
}

export class AuthStoreRec extends Record({
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

export default Store({

  getInitialState() {
    return new AuthStoreRec()
  },

  initialize() {
    this.on(AT.UPDATE_CONNECTORS, (state, items) => state.upsertItems(items) );
    this.on(AT.RECEIVE_CONNECTORS, (state, items) => state.setItems(items) );
    this.on(AT.DELETE_CONN, (state, id) => state.remove(id));
  }
})

