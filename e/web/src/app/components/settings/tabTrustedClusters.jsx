import React from 'react';
import connect from 'telebase-app/components/connect';
import getters from 'app/flux/settingsClusters/getters';
import userAclGetters from 'telebase-app/flux/userAcl/getters';
import * as actions from 'app/flux/settingsClusters/actions';
import ConfigItemList from './configItemList';
import {openDeleteDialog} from 'app/flux/settingsDialogs/actions';
import ConfigDeleteDialog from './configDeleteDialog';
import { EmptyList } from './elements';
import ConfidAddEdit from './configAddEdit';
import ChangeTracker from './../changeTracker';
import { trustedClusterTemplate } from './examples';
  
class TrustedClusters extends React.Component {

  state = {}

  onNewItem = () => { 
    this.refTracker.checkIfUnsafedData(()=> {
      let newItem = this.props.store.createItem();
      newItem = newItem.setContent(trustedClusterTemplate);      
      actions.setCurCluster(newItem); 
    })
  }

  onItemClick = item => {
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurCluster(item);    
    })     
  }
  
  onItemDelete = () => {
    openDeleteDialog(this.props.store.curItem);
  }

  onCancelNewItem = () => {    
    actions.setCurCluster();
  }

  onItemSave = item => {    
    actions.saveCluster(item);
  }
      
  componentDidMount(){    
    actions.setCurCluster();
  }
  
  render() {    
    const { store, saveAttempt, userAclStore } = this.props;                
    const curItem = store.getCurItem();
    const items = store.getItems();
    const access = userAclStore.getClusterAccess();
    const canCreate = access.create;

    const props = {
      ref: e => this.refTracker = e,
      className: "grv-settings-tab",
      route: this.props.route
    }

    if(!curItem){
      return (
        <ChangeTracker {...props}>         
          <EmptyList canCreate={canCreate} onClick={this.onNewItem}/>
        </ChangeTracker>
      )
    }
                
    return (      
      <ChangeTracker {...props}> 
        { !curItem.isNew &&
        <ConfigItemList          
          canCreate={canCreate}
          btnText="New Trusted Cluster"
          curItem={curItem}
          items={items}          
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}                        
        />      
        }
        <ConfidAddEdit           
          onCancel={this.onCancelNewItem}
          onDelete={this.onItemDelete}
          onSave={this.onItemSave}
          access={access}
          item={curItem} 
          saveAttempt={saveAttempt}/>        
        <ConfigDeleteDialog onContinue={actions.deleteCluster} />                          
      </ChangeTracker>              
    );
  }    
}

function mapStateToProps() {
  return {    
    userAclStore: userAclGetters.userAcl,
    saveAttempt: getters.saveAttempt,
    store: getters.store
  }  
}

export default connect(mapStateToProps)(TrustedClusters);

