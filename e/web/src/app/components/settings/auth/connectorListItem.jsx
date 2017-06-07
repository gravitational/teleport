import $ from 'jQuery';
import React from 'react';
import ace from 'brace';
import 'brace/mode/json';

import { Select } from 'app/components/common/select';
import Box from 'app/components/common/boxes/box';
import Button from 'app/components/common/button';
import Layout from 'app/components/common/layout';
import Form from 'app/components/common/form';
import actions from 'app/flux/settingsAuth/actions';

import {
  isValidOidcScope,
  isValidRoleMapping,
  parseRoleMapping,
  toJSON,
  roleEmptyPlaceHolder
} from './utils'

const { UndoManager } = ace.acequire('ace/undomanager');

const Label = ({text}) => ( 
  <label style={{ width: "120px" }} className="text-bold m-t-xs"> {text} </label>
)

const Description = ({text}) => (
  <div className="help-block m-t-xs" >{text}</div>
)

const inputSmallStyleMaxWidth = {
  maxWidth: "400px"
}

const inputSmallStyle = {
  flex: "1",  
  ...inputSmallStyleMaxWidth
}

const styleStretch = {
  flex: "1",
  maxWidth: "700px"
}

const parseScopeStr = str => {
  str = str || '';
  return str.split(/,| / ).filter(i => i.length > 0);
}

const ConnectorListItem = React.createClass({
      
  componentDidMount() {        
    this.context.register(this);

    $.validator.addMethod("grv-oidc-scope", function (value) {
      if (value.length === 0) {
        return true;
      }       
      let isValid = value.split(',').every(isValidOidcScope);
      return isValid;
    }, 'Please enter a valid scope name.');

    $.validator.addMethod("grv-oidc-role-mapping", function (value) {      
      if (!value) {
        return true;
      }

      try {
        value = JSON.parse(value);        
        return isValidRoleMapping(value);
      } catch (err) {
        return false;
      }
            
    }, 'Please enter a valid role mapping.');

    $(this.refForm).validate({
      rules: {
        jsonEditor: {
          'grv-oidc-role-mapping': true
        },
        connectorname:{
          'required': true          
        },
        scopes: {
          'grv-oidc-scope': true          
        }
      }
    })
  },

  componentWillUnmount() {
    this.context.unregister(this);
  },
  
  calcState(connector) {
    this.snapshot = JSON.stringify(connector);
    this.isDirty = false;    
    return ({            
      ...connector
    })
  },

  getInitialState() {    
    return this.calcState(this.props.connector);    
  },
  
  handleIsDirty(){
    this.isDirty = JSON.stringify(this.state) !== this.snapshot;
  },

  hasChanges(){
    return this.isDirty || this.state.isNew;
  },

  onSave() {
    $(this.refForm).validate().settings.ignore = [];
    if ($(this.refForm).valid()) {      
      this.setState({
        isSaving: true
      });
      
      actions.save(this.state).fail(() => {
        this.setState({
          isSaving: false
        })
      });      
    } 
  },

  onChangeName(e){  
    this.setState({
      id: e.target.value
    })
  },

  onChangeDisplayName(e){
    this.setState({
      displayName: e.target.value
    });
  },

  onChangeClientId(e){  
    this.setState({
      clientId: e.target.value
    })
  },

  onChangeClientSecret(e){
    this.setState({
      clientSecret: e.target.value
    })
  },

  onChangeIssuerUrl(e){  
    this.setState({
      issuerUrl: e.target.value
    })
  },

  onChangeScope(newValue){      
    newValue = parseScopeStr(newValue);                                                       
    this.setState({scope: newValue})
  },

  onChangeRedirectUrl(e){
    this.setState({
      redirectUrl: e.target.value  
    });
  },
  
  onChangeMapping(str){  
    let roleMapping = parseRoleMapping(str) || [];    
    this.setState({
      roleMapping 
    });    
  },
    
  onBlur(e){        
    let valueToAppend = parseScopeStr(e.target.value);
    let scope = [...this.state.scope, ...valueToAppend]; 
    this.setState({
      scope
    });        
  },
        
  render() {      
    let {    
      isSaving,
      id,      
      displayName,
      clientId,
      clientSecret,
      roleMapping,
      scope,
      redirectUrl,
      issuerUrl } = this.state;
    
    let {
        onCancelNew,
        connector } = this.props;
      
    this.handleIsDirty();
            
    let { isNew } = connector;
                      
    let jsonStr = toJSON(roleMapping);
          
    let isPrimaryBtnEnabled = this.hasChanges();
  
    let $secondaryBtn = null;
    if (isNew) {
      $secondaryBtn = (
        <Button className="btn-default"
          isDisabled={isSaving}
          onClick={onCancelNew} >
          Cancel
        </Button>
      );               
    } else {
      $secondaryBtn = (
        <Button className="btn-danger"
          onClick={() => actions.openDeleteConnectorDialog(id)} >
          Delete
        </Button>
      );                 
    } 

    return (      
      <Form refCb={e => this.refForm = e} className="grv-settings-auth-connector-item m-b-lg">
        <Box.Header>
          <h3>OpenID Connect</h3>
        </Box.Header>                          
        <div className="p-w-xs m-b">
          <Layout.Flex dir="row" className="m-t-md">
            <Label text="Name:" />                                
            <div style={{flex: "1"}}>
              <input
                autoFocus
                autoComplete="off"
                style={inputSmallStyle}
                name="connectorname"
                value={id}
                onChange={this.onChangeName}
                type="text"
                disabled={!isNew}
                className="form-control"
                placeholder="github" />              
            </div>                                                                    
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Display Name:" />                                
            <div style={styleStretch}>              
              <input required
                placeholder="GitHub"
                name="displayname"
                autoComplete="off"
                value={displayName}
                style={inputSmallStyleMaxWidth}
                onChange={this.onChangeDisplayName}
                type="text" className="form-control" />
              <Description text={`Connector display name is shown during user login as in "Login with X" button`} />
            </div>                                                                                                            
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t-md">
            <Label text="Client ID:" />                                
            <div style={styleStretch}>
              <input required
                name="clientid" 
                autoComplete="off"
                value={clientId}
                onChange={this.onChangeClientId}
                type="text" className="form-control" placeholder="s6BhdRkqt3"/>
            </div>                                                                                               
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Client Secret:" />                                
            <div style={styleStretch}>
              <input required
                name="clientsecret"
                autoComplete="off"
                value={clientSecret}
                onChange={this.onChangeClientSecret}
                type="text" className="form-control" placeholder="1912308409123890"/>
            </div>                                                                                    
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Issuer URL:" />                                
            <div style={styleStretch}>
              <input 
                name="ussuerurl"                
                type="text"
                autoComplete="off"
                value={issuerUrl}
                onChange={this.onChangeIssuerUrl}
                className="form-control"
                placeholder="https://idp.example.org/SAML2"/>
            </div>                                                                                    
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Redirect URL:" />                                
            <div style={styleStretch}>
              <input required
                name="redirecturl" 
                type="text"
                autoComplete="off"
                className="form-control"
                value={redirectUrl}
                onChange={this.onChangeRedirectUrl}
                placeholder="https://sp.example.org/SAML2/SSO/Artifact?SAMLart=artifact"/>
            </div>                                                                                    
          </Layout.Flex>                    
          <Layout.Flex dir="row" className="m-t">
            <Label text="Scope:" />
            <div style={styleStretch} className="border">
              <Select
                className="grv-settings-label-selector"
                searchable multi simpleValue allowCreate clearable={false}
                value={scope}
                classRules="required"
                onBlur={this.onBlur}
                placeholder="scope1 scope2 scope3"
                onChange={this.onChangeScope} />
            </div>
          </Layout.Flex>
          <Layout.Flex dir="row" className="m-t">
            <Label text="Role Mappings:" />
            <div style={styleStretch}>
              <JsonEditor
                json={jsonStr}
                onChange={this.onChangeMapping}
                name="rolemappings" required
                type="text"
              />
            </div>
          </Layout.Flex>           
        </div>
        <div className="m-t-sm">
          <Button                    
            onClick={this.onSave}      
            isDisabled={!isPrimaryBtnEnabled}
            isProcessing={isSaving}
            className="btn-primary m-r-sm">
            Save
          </Button>
          {$secondaryBtn}          
        </div>   
      </Form>
    );
  }
});

ConnectorListItem.contextTypes = {
  register: React.PropTypes.func.isRequired,
  unregister: React.PropTypes.func.isRequired
};

let JsonEditor = React.createClass({

  ensurePlaceHolder() {
    let shouldShow = !this.editor.session.getValue().length;
    let renderer = this.editor.renderer;
    let node = renderer.emptyMessageNode;
    if (!shouldShow && node) {
      renderer.scroller.removeChild(renderer.emptyMessageNode);
      renderer.emptyMessageNode = null;
    } else if (shouldShow && !node) {
      node = renderer.emptyMessageNode = document.createElement("div");      
      node.textContent = JSON.stringify(roleEmptyPlaceHolder, null, '  ');
      node.className = "ace_invisible ace_emptyMessage";
      node.style.padding = "0 9px";
      renderer.scroller.appendChild(node);
    }
  },

  initEditSessions() {
    let { json } = this.props;
    let undoManager = new UndoManager();  
    json = json || '';      
    this.isDirty = false;
    this.session = new ace.EditSession(json);      
    this.session.setOptions({ tabSize: 2, useSoftTabs: true });
    this.session.setUndoManager(undoManager);
    this.session.setUseWrapMode(true)        
    this.session.setMode("ace/mode/json");  
    this.editor.setSession(this.session);
    this.editor.renderer.setShowGutter(true);
  },

  onChange() {    
    let value = this.session.getValue();    
    this.refValidation.value = value;
    if (this.props.onChange) {
      this.props.onChange(value);
    }      
  },

  componentDidMount() {
    this.editor = ace.edit(this.refs.ace_viewer);    
    this.editor.renderer.setShowGutter(false);
    this.editor.renderer.setShowPrintMargin(false);
    this.editor.renderer.setOption('showLineNumbers', false)
    this.editor.setFadeFoldWidgets(true);
    this.editor.setWrapBehavioursEnabled(true);
    this.editor.setHighlightActiveLine(false);
    this.editor.setShowInvisibles(false);      
    this.editor.setReadOnly(false);    
    this.editor.on('input', this.onChange);
    this.initEditSessions(this.props.initialData);

    this.ensurePlaceHolder();    
    this.editor.on("input", this.ensurePlaceHolder);
  },

  componentWillUnmount() {
    this.editor.destroy();
    this.editor = null;
    this.session = null;    
  },
    
  shouldComponentUpdate() {
    return false;
  },

  render() {    
    return (      
      <div>
        <div className="grv-settings-json-editor">            
          <div ref="ace_viewer" style={editorStyle}></div>                  
        </div>        
        <Description text={`Define the scopes to request from the OIDC provider and how to map the returned Claims to Roles. Each claim can be mapped to multiple roles.`} />
        <input ref={e => this.refValidation = e} type="hidden" name="jsonEditor" />
      </div>                  
    )
  }
});

const editorStyle = {
  position: 'absolute',
  top: '0px',
  right: '0px',
  bottom: '0px',
  left: '0px'
};

export default ConnectorListItem;
