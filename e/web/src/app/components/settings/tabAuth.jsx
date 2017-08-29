import React from 'react';
import connect from 'telebase-app/lib/connect';
import getters from 'app/flux/settingsAuth/authGetters';
import {openDeleteDialog} from 'app/flux/settings/actions';
import * as Alerts from 'app/components/common/alerts';
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
    const { store, saveAttempt, errors } = this.props;                
    const curItem = store.getCurItem();
    const items = store.getItems();   
        
    const $errors = errors.map( ( text, i ) => (
      <Alerts.Danger key={i} className="m-b-sm">
        {text}
      </Alerts.Danger>  
    ));
        
    if(!curItem && $errors !== 2){
      return (
        <div className="grv-settings-tab-auth">                                             
          <div>
            {$errors}
          </div>
          <EmptyList onClick={this.onNewItem}/>
        </div>
      )
    }
               
    const displayItemList = !!curItem && !curItem.isNew;
    const displayYamlEditor = !!curItem;

    return (                                              
      <ChangeTracker ref={ e => this.refTracker = e } className="grv-settings-tab-auth" route={this.props.route}>   
        <div>
          {$errors}
        </div>
        <div style={s}>
          { displayItemList &&
          <ConfigItemList    
            btnText="New Connector"          
            curItem={curItem}
            items={items}          
            onNew={this.onNewItem}
            onItemClick={this.onItemClick}                        
          />      
          }
          { displayYamlEditor &&
          <ConfidAddEdit           
            key={curItem.key}          
            onCancel={this.onCancelNewItem}
            onDelete={this.onItemDelete}
            onSave={this.onItemSave}
            item={curItem} 
            saveAttempt={saveAttempt}/>        
          }
          <ConfigDeleteDialog onContinue={actions.deleteAuthProvider } />                                  
        </div>
      </ChangeTracker>              
    );
  }    
}

const s = {
  display: "flex",
  height: "100%"
}

function mapStateToProps() {
  return {    
    saveAttempt: getters.saveAttempt,
    errors: getters.errors,
    store: getters.store
  }  
}

export default connect(mapStateToProps)(Auth);

