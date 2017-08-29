import React from 'react';
import connect from 'telebase-app/lib/connect';
import getters from 'app/flux/settingsRoles/getters';
import {openDeleteDialog} from 'app/flux/settings/actions';
import * as actions from 'app/flux/settingsRoles/actions';
import ConfigItemList from './configItemList';
import ConfigDeleteDialog from './configDeleteDialog';
import { EmptyList } from './emptyCfg';
import ConfidAddEdit from './configAddEdit';
import ChangeTracker from './../changeTracker';

class Roles extends React.Component {

  state = {}

  onNewItem = () => {    
     this.refTracker.checkIfUnsafedData(()=> {
        actions.setCurRole(this.props.store.createItem())    
     });
  }
  
  onCancelNewItem = () => {    
    actions.setCurRole();
  }

  onItemSave = item => {    
    actions.saveRole(item);
  }

  onItemClick = item => {            
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurRole(item);   
    })    
  }

  onItemDelete = () => {
    openDeleteDialog(this.props.store.curItem);
  }
    
  componentDidMount(){    
    actions.setCurRole();
  }
    
  render() {    
    const { store, saveAttempt, changeTracker } = this.props;                
    const curItem = store.getCurItem();
    const items = store.getItems();
    
    if(!curItem){
      return (
        <div className="grv-settings-tab">                                             
          <EmptyList onClick={this.onNewItem}/>
        </div>
      )
    }
                    
    return (      
      <ChangeTracker ref={ e => { this.refTracker = e } } 
        className="grv-settings-tab" 
        route={this.props.route}>   
        { !curItem.isNew &&
        <ConfigItemList        
          btnText="New Role"                        
          curItem={curItem}
          items={items}          
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}                        
        />      
        }
        <ConfidAddEdit 
          key={curItem.key}
          changeTracker={changeTracker}
          onCancel={this.onCancelNewItem}
          onDelete={this.onItemDelete}
          onSave={this.onItemSave}
          item={curItem} 
          saveAttempt={saveAttempt}/>        
        <ConfigDeleteDialog onContinue={actions.deleteRole} />                          
      </ChangeTracker>              
    );
  }    
}

function mapStateToProps() {
  return {    
    saveAttempt: getters.saveAttempt,
    store: getters.store
  }  
}

export default connect(mapStateToProps)(Roles)
