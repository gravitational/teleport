import React from 'react';
import classnames from 'classnames';
import { toArray } from 'lodash';
import reactor from  'app/reactor';
import Layout from 'app/components/common/layout';
import getters from 'app/flux/settingsRoles/getters';
import actions from 'app/flux/settingsRoles/actions';
import RoleDetails from './roleDetails';
import ChangeTracker from './../changeTracker';
import DeleteRoleDialog from './roleDeleteDialog';

let RoleList = React.createClass({
  
  renderItem(name, displayName, key) {
    let { role, onSelectRole } = this.props;
    let isSelected = role && role.name === name && !role.isNew;
    let className = classnames('grv-settings-roles-group-menu-item', {
      'active': isSelected
    });

    return (
      <li key={key} className={className} onClick={() => onSelectRole(name)}>
        <a>
          <span> {displayName } </span>
        </a>
      </li>
    )
  },

  render() {        
    let { onNew, roles } = this.props;    
    let $roles = roles.map((r, key) => this.renderItem(r.name, r.displayName, key));
    
    return (
      <div className="grv-settings-roles-group">
        <ul className="grv-settings-roles-group-menu">
          {$roles}
        </ul>         
        <div className="text-right m-t-sm">
          <button onClick={onNew} className="btn btn-sm btn-default"> New Role</button>
        </div>  
      </div>
    );
  }
});

const EmptyList = React.createClass({
  render() {
    return (
      <div className="m-t text-center text-muted" style={{ minHeight: "50px" }}>
        <p>
          <strong> You have no roles. Please use 'New Role' button to create one.</strong>
        </p>                
    </div>  
    )  
  }
});

const Roles = React.createClass({

  mixins: [reactor.ReactMixin],
  
  getDataBindings() {
    return {
      deleteAttemp: getters.deleteRoleAttemp,
      store: getters.roleStore
    }
  },
  
  componentWillUnmount() {
    actions.clear();    
    
  },
  
  componentDidMount() {
    actions.setSelectedRole();    
  },
  
  onSelectRole(roleName){        
    actions.setSelectedRole(roleName);          
  },
  
  render() {
    let { store, deleteAttemp } = this.state;
    let { allRoles,selectedRole, roleToDelete } = store;          
    let allRoleNames = toArray(allRoles).map(r => r.name);          
    return (
      <div className="m-t grv-settings-roles">
        <ChangeTracker
          router={this.props.router}  
          route={this.props.route}
        >                          
          <DeleteRoleDialog
            onContinue={() => actions.deleteRole(selectedRole.name)}        
            onCancel={actions.closeDeleteRoleDialog}        
            roleName={roleToDelete}
            attemp={deleteAttemp}
          />        
          <Layout.Flex dir="row">
            <RoleList
              role={selectedRole}          
              roles={allRoles}          
              onNew={actions.newRole}
              onSelectRole={this.onSelectRole}                        
            />          
            <div className="m-l" style={{ width: '100%' }}>
              {selectedRole ?
                <RoleDetails key={selectedRole.key} roleNames={allRoleNames} role={selectedRole} />
                : <EmptyList/>
              }
            </div>  
          </Layout.Flex>    
        </ChangeTracker>  
      </div>  
    );
  }
});

export default Roles;