import React from 'react';
import connect from 'telebase-app/lib/connect';
import getters from 'app/flux/settingsClusters/getters';
import * as actions from 'app/flux/settingsClusters/actions';
import ConfigItemList from './configItemList';
import {openDeleteDialog} from 'app/flux/settings/actions';
import ConfigDeleteDialog from './configDeleteDialog';
import { EmptyList } from './emptyCfg';
import ConfidAddEdit from './configAddEdit';
import ChangeTracker from './../changeTracker';
  
class TrustedClusters extends React.Component {

  state = {}

  onNewItem = () => { 
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurCluster(this.props.store.createItem())    
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
    const { store, saveAttempt } = this.props;                
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
      <ChangeTracker ref={ e => this.refTracker = e } className="grv-settings-tab" route={this.props.route}> 
        { !curItem.isNew &&
        <ConfigItemList          
          btnText="New Trusted Cluster"
          curItem={curItem}
          items={items}          
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}                        
        />      
        }
        <ConfidAddEdit 
          key={curItem.key}
          onCancel={this.onCancelNewItem}
          onDelete={this.onItemDelete}
          onSave={this.onItemSave}
          item={curItem} 
          saveAttempt={saveAttempt}/>        
        <ConfigDeleteDialog onContinue={actions.deleteCluster} />                          
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

export default connect(mapStateToProps)(TrustedClusters);

