import { Record, Map } from 'immutable';
import { isString } from 'lodash';

export class ItemRec extends Record({
  isNew: false,
  kind: '',
  key: '',
  name: '',
  displayName: '',
  content: ''
}){
  constructor(props={}){
    const key = Math.random().toString();

    super({
      key,
      displayName: props.name,
      ...props
    })
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

export class StoreRec extends Record({
  curItem: null,
  itemToDelete: null,
  items: Map()
}){
    
  getItemToDelete(){
    return this.get('itemToDelete');
  }

  getItems(){
    return this.items.valueSeq();
  }
  
  getCurItem(){    
    return this.get('curItem');
  }

  upsertItems(jsonItems){
    let itemMap = this.get('items');
    jsonItems.forEach(json => {
      const rec = new ItemRec(json);
      itemMap = itemMap.set(rec.getName(), rec)
    })

    return this.set('items', itemMap);
  }

  findItem(item /* string|itemRec*/){    
    if(!item) {
      return null;
    }

    const name = isString(item) ? item : item.getName();    
    return this.getIn(['items', name]);    
  }

  removeAll(){
    return this.set('items', new Map())
  }

  createItem(){
    return new ItemRec({ isNew: true });
  }

  setItems(jsonItems){
    let store = this.removeAll();
    return store.upsertItems(jsonItems);            
  }  
  
  setItemToDelete(id) {
    return this.set('itemToDelete', id);
  }

  setCurItem(item/* string|itemRec*/){        
    if(item && item.isNew ){
      return this.set('curItem', item);        
    }

    const found = this.findItem(item);    
    if (!found) {
      const firstAvailable = this.get('items').first();
      if (firstAvailable) {
        return this.setCurItem(firstAvailable);  
      }else{
        return this.set('curItem', null);
      }            
    }
    
    return this.set('curItem', found);      
  }
}
