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
      ...props
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

export class StoreRec extends Record({
  curItem: null,
  items: Map()
}){
      
  getItems(){
    return this.items.valueSeq().sortBy(i => i.getName());
  }
  
  getCurItem(){    
    return this.get('curItem');
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

  getNext(id){
    const curItem = this.findItem(id);            
    if(!curItem){
      return null;
    }

    const itemList = this.getItems();
    const index = itemList.indexOf(curItem);      

    if(itemList.size === 1){
      return null;
    }

    if(index === 0){
      return itemList.get(1);
    }

    if(index === itemList.size -1){
      return itemList.get(index-1);
    }
    
    return itemList.get(index+1);              
  }

  createItem(){
    return new ItemRec({ isNew: true });
  }

  setItems(jsonItems){
    let store = this.removeAll();
    return store.upsertItems(jsonItems);            
  }  
    
  setCurItem(item/* string|itemRec*/){        
    if(item && item.isNew ){
      return this.set('curItem', item);        
    }

    const found = this.findItem(item);    
    if (!found) {
      const firstAvailable = this.getItems().first();
      if (firstAvailable) {
        return this.setCurItem(firstAvailable);  
      }else{
        return this.set('curItem', null);
      }            
    }
    
    return this.set('curItem', found);      
  }
}
