import React from 'react';
import $ from 'jQuery';
import actions from 'app/flux/settingsRoles/actions';
import Layout from 'app/components/common/layout';
import Box from 'app/components/common/boxes/box';
import Button from 'app/components/common/button';
import RoleSessionAccess from './roleDetailsSession';
import RoleAdmin from './roleDetailsAdmin';
import classnames from 'classnames';
import Form from 'app/components/common/form';
import { UserRoleSystemNameEnum } from 'app/services/enums'
import { durationToStr, parseDuration, parseNodeLabel, isValidRoleName } from 'app/lib/paramUtils';

const VALIDATION_UNIQUE_ROLE_NAME = 'name already exists'
const VALIDATION_VALID_ROLE_NAME = 'invalid name'

const equalString = (a, b) => {
  a = a.toString();
  b = b.toString();
  b = b.toLocaleUpperCase().trim();
  a = a.toLocaleUpperCase().trim();
  return a === b
}

const fromMsToNano = ms => ms * 1000000;
const fromNanoToMs = nano => nano / 1000000;

const HeaderLayout = props => (
  <Layout.Flex className="m-b" dir="column" justify="between" style={{ flex: "1" }} align="center">
    {props.children}
  </Layout.Flex>
)

const RoleDetails = React.createClass({    
    
  calcState(role) {          
    let state = { 
      ...role      
     };                
    let ssh = state.access.ssh;
    let nodeLabels = ssh.nodeLabels || {};
    let maxTtl = ssh.maxTtl;
    let logins = ssh.logins;
    
    ssh.maxTtlText = durationToStr(fromNanoToMs(maxTtl));
    ssh.enabled = state.isNew || !!logins && logins.length > 0;
    ssh.nodeLabelsText = Object.getOwnPropertyNames(nodeLabels)
      .map(name => `${name}=${nodeLabels[name]}`)
      .join(',');
            
    this.snapshot = JSON.stringify(state);
    this.isDirty = false;        
    return state;
  },

  getInitialState() {      
    return this.calcState(this.props.role);  
  },
    
  componentWillUnmount() {
    this.context.unregister(this);
  },

  componentDidMount() {    
    this.context.register(this);
    $.validator.addMethod("grvValidRoleName", value => {      
      let { roleNames } = this.props; 
      if (value === UserRoleSystemNameEnum.ADMIN) {
        return false;
      }

      let isUnique = roleNames.some(i => equalString(i, value))      
      return !isUnique;
    }, VALIDATION_UNIQUE_ROLE_NAME);

    $.validator.addMethod("grvUniqueRoleName", value => {                         
      return isValidRoleName(value);
    }, VALIDATION_VALID_ROLE_NAME);    
  },
    
  receiveFormRef(e) {      
    $(e).validate({
      rules:{
        rolename:{          
          required: true,
          grvUniqueRoleName: true,
          grvValidRoleName: true
        }
      }
    })    
  },
    
  prepareToSave() {
    let ssh = this.state.access.ssh;                
    if (ssh.enabled) {      
      let maxTtl = parseDuration(ssh.maxTtlText);
      ssh.nodeLabels = parseNodeLabel(ssh.nodeLabelsText);              
      ssh.maxTtl = fromMsToNano(maxTtl);            
    }else{      
      ssh.logins = [];
      ssh.nodeLabels = {};
    }          
  },

  handleIsDirty(){
    this.isDirty = JSON.stringify(this.state) !== this.snapshot;
  },

  hasChanges(){
    return this.isDirty === true || this.state.isNew;
  },
  
  onSave() {
    let $forms = $('form');
    let isValid = true;
    for(let i = 0; i < $forms.length; i++){
      $forms.eq(i).validate().settings.ignore = [];
      isValid = $forms.eq(i).valid() && isValid;
    }

    if (isValid) {
      this.prepareToSave();      
      
      this.setState({
          isSaving: true
        });

      actions.saveRole(this.state).fail(()=>{
        this.setState({
          isSaving: false
        });
      })
    }    
  },
  
  onChangeName(e) {    
    this.setState({
      name: e.target.value
    })
  },

  onChangeAdmin(value) {
    let { access } = this.state;                 
    access.admin.enabled = value;
    this.setState({
      access
    })
  },

  onChangeSession(ssh){
    let { access } = this.state;                 
    access.ssh = ssh;
    this.setState({
      access
    })    
  },
  
  renderViewHeader(roleName) {    
    return (
      <div className="grv-settings-role-details-name">        
        {roleName}
      </div>  
    )  
  },

  renderEditHeader(roleName) {    
    return (
      <HeaderLayout>
        <Form refCb={this.receiveFormRef} className="m-b-xs" ref="form" style={{ width: '100%' }}>          
          <input
            autoFocus
            type="text"
            className="form-control grv-settings-role-details-name-input"
            placeholder="Enter Rule Name"
            onChange={this.onChangeName}
            defaultValue={roleName}                                
            name="rolename"/>          
        </Form>  
      </HeaderLayout>
    )
  },
  
  renderContent(){
    let { access, system } = this.state;         
    return (
      <div className="m-t-md">     
        <RoleAdmin          
          editable={!system}
          enabled={access.admin.enabled}
          onChange={this.onChangeAdmin} />
        <RoleSessionAccess          
          editable={!system}
          ssh={access.ssh}                              
          onChange={this.onChangeSession} />                          
      </div>        
    )        
  },

  renderFooter() {    
    this.handleIsDirty();
    let { isNew, isSaving, system, name } = this.state;    
    let secondaryBtnCb = isNew ? actions.cancelNewRole : () => actions.openDeleteRoleDialog(name);
    let secondaryBtnText = isNew ? 'Cancel' : 'Delete';
    let secondaryBtnClassName = classnames({
      'btn-danger': !isNew,
      'btn-default': isNew
    });
          
    let isPrimaryBtnEnabled = isNew || this.isDirty;

    return (
      <div className="m-t-lg">
        <Button          
          onClick={this.onSave}
          isProcessing={isSaving}
          isDisabled={!isPrimaryBtnEnabled}
          className="btn-primary m-r-sm">
          Save
        </Button>
        <Button
          className={secondaryBtnClassName}
          onClick={secondaryBtnCb}
          isDisabled={isSaving || system}>            
          {secondaryBtnText}
        </Button>
      </div>     
    )
  },

  render() {
    let { name, displayName, isNew } = this.state;        
    let $header = null;
        
    if (isNew) {
      $header = this.renderEditHeader(name);
    } else {
      $header = this.renderViewHeader(displayName);  
    }

    let $content = this.renderContent();
    let $footer = this.renderFooter();
                         
    return (
      <Box className="grv-settings-role-details">                             
        {$header}        
        {$content}      
        {$footer}                                                    
      </Box>
    );
  }
});

RoleDetails.contextTypes = {
  register: React.PropTypes.func.isRequired,
  unregister: React.PropTypes.func.isRequired
};

export default RoleDetails;