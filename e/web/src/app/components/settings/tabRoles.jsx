import React from 'react';
import connect from 'telebase-app/components/connect';
import userAclGetters from 'telebase-app/flux/userAcl/getters';
import getters from '../../flux/settingsRoles/getters';
import {openDeleteDialog} from '../../flux/settingsDialogs/actions';
import * as actions from '../../flux/settingsRoles/actions';
import ConfigItemList from './configItemList';
import ConfigDeleteDialog from './configDeleteDialog';
import { EmptyList } from './elements';
import ConfidAddEdit from './configAddEdit';
import ChangeTracker from './../changeTracker';
import { roleTemplate } from './examples';

class Roles extends React.Component {

  state = {}

  onNewItem = () => {
    this.refTracker.checkIfUnsafedData(()=> {
      let newItem = this.props.store.createItem();
      newItem = newItem.setContent(roleTemplate);
      actions.setCurRole(newItem);
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
    const { store, saveAttempt, userAclStore } = this.props;
    const curItem = store.getCurItem();
    const items = store.getItems();
    const access = userAclStore.getRoleAccess();
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
          btnText="New Role"
          curItem={curItem}
          items={items}
          onNew={this.onNewItem}
          onItemClick={this.onItemClick}
        />
        }
        <ConfidAddEdit
          access={access}
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
    userAclStore: userAclGetters.userAcl,
    saveAttempt: getters.saveAttempt,
    store: getters.store
  }
}

export default connect(mapStateToProps)(Roles)
