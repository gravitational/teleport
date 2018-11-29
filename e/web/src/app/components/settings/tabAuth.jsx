import React from 'react';
import connect from 'telebase-app/components/connect';
import userAclGetters from 'telebase-app/flux/userAcl/getters';
import getters from '../../flux/settingsAuth/authGetters';
import {openDeleteDialog} from '../../flux/settingsDialogs/actions';
import * as actions from '../..//flux/settingsAuth/actions';
import ConfigItemList from './configItemList';
import ConfigDeleteDialog from './configDeleteDialog';
import ConfidAddEdit from './configAddEdit';
import { EmptyList } from './elements';
import ChangeTracker from './../changeTracker';
import { authTemplate } from './examples';

class Auth extends React.Component {

  state = {}

  onNewItem = () => {
    this.refTracker.checkIfUnsafedData(()=> {
      let newItem = this.props.store.createItem();
      newItem = newItem.setContent(authTemplate);
      this.onItemClick(newItem);
    });
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
    const { store, saveAttempt, userAclStore } = this.props;
    const curItem = store.getCurItem();
    const items = store.getItems();
    const access = userAclStore.getConnectorAccess();
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

    const displayItemList = !!curItem && !curItem.isNew;
    const displayYamlEditor = !!curItem;

    return (
      <ChangeTracker {...props}>
        { displayItemList &&
        <ConfigItemList
          canCreate={canCreate}
          btnText="New Connector"
          curItem={curItem}
          items={items}
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}
        />
        }
        { displayYamlEditor &&
        <ConfidAddEdit
          access={access}
          onCancel={this.onCancelNewItem}
          onDelete={this.onItemDelete}
          onSave={this.onItemSave}
          item={curItem}
          saveAttempt={saveAttempt}/>
        }
        <ConfigDeleteDialog onContinue={actions.deleteAuthProvider } />
      </ChangeTracker>
    );
  }
}

function mapStateToProps() {
  return {
    saveAttempt: getters.saveAttempt,
    store: getters.store,
    userAclStore: userAclGetters.userAcl
  }
}

export default connect(mapStateToProps)(Auth);