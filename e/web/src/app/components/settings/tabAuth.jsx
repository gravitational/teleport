import React from 'react';
import connect from 'telebase-app/lib/connect';
import getters from 'app/flux/settingsAuth/authGetters';
import {openDeleteDialog} from 'app/flux/settings/actions';
import * as actions from 'app/flux/settingsAuth/actions';
import ConfigItemList from './configItemList';
import ConfigDeleteDialog from './configDeleteDialog';
import ConfidAddEdit from './configAddEdit';
import { EmptyList } from './emptyCfg';
import ChangeTracker from './../changeTracker';

class Auth extends React.Component {

  state = {}

  onNewItem = () => {    
    actions.setCurProvider(this.props.store.createItem())    
  }
  
  onCancelNewItem = () => {    
    actions.setCurProvider();
  }

  onItemSave = item => {    
    actions.saveAuthProvider(item);
  }

  onItemClick = item => {    
    this.refTracker.checkIfUnsafedData(()=> {
      actions.setCurProvider(item);    
    }) 
  }

  onItemDelete = () => {
    openDeleteDialog(this.props.store.curItem);
  }
    
  componentDidMount(){    
    actions.setCurProvider();
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
          btnText="New Connector"          
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
        <ConfigDeleteDialog onContinue={actions.deleteAuthProvider } />                                  
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

export default connect(mapStateToProps)(Auth);

