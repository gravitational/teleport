import React from 'react';
import $ from 'jQuery';
import { Select } from 'app/components/common/select';
import Layout from 'app/components/common/layout';
import Form from 'app/components/common/form';
import Separator from 'app/components/common/separator';
import { isValidLoginName, parseDuration, parseNodeLabel } from 'app/lib/paramUtils';
import { SshBuiltInLoginEnum } from 'app/services/enums'

const MIN_ALLOWED_DURATION = 600000 * 6 /* 1h */
const DESC = 'Enabling this option will allow users to SSH into the servers of this cluster. You must restrict the OS logins a user will be allowed to lohin as.';
const LOGIN_DESC = 'This option specifies the host-local OS logins the group members can login as.';
const MAX_SESSION_DESC = 'This option specifies the expiration time of the session keys. When a key is expired, a user will be prompted to re-login.';
const INVALID_DUR_ERROR = 'Please enter a valid duration.';
const NODE_LABELS_DESC = 'Members of this group will only have access to the nodes with the specified labels. Access to any other node will be denied.';
const INVALID_DUR_RANGE_ERROR = 'Minimum allowed value is 1h';

const parseLoginStr = str => {
  str = str || '';
  return str.split(/,| / ).filter(isValidLoginName);
}

const defaultLoginOptions = [
  {
    value: SshBuiltInLoginEnum.ROOT,
    label: 'root'
  }
];
  
const defaultLoginDescription = {
  [SshBuiltInLoginEnum.ROOT]: 'Full access'    
}

const Label = props => (
  <div style={{ width: "165px" }}>
    {props.children}
  </div>
)
  
const RoleSshAccess = React.createClass({
    
  componentDidMount() {            
    $.validator.addMethod('grv-settings-ttl-duration', function (value) {            
      try {        
        parseDuration(value);        
      } catch (err) {
        return false;
      }

      return true;
    }, INVALID_DUR_ERROR);

    $.validator.addMethod('grv-settings-ttl-duration-range', function (value) {            
      try {        
        let ms = parseDuration(value);        
        if(ms < MIN_ALLOWED_DURATION){
          return false;
        }
      } catch (err) {        
        return true;
      }      

      return true;
    }, INVALID_DUR_RANGE_ERROR);

    $.validator.addMethod('grv-settings-node-labels', function (value) {                    
      try {
        parseNodeLabel(value)
      } catch (err) {
        return false;
      }

      return true;            
    }, 'Please enter valid node label');
                
    $(this.refForm).validate({
      rules: {        
        ttlDuration:{
          'grv-settings-ttl-duration': true,
          'grv-settings-ttl-duration-range': true
        },
        grvNodeLabels:{
          'grv-settings-node-labels': true        
        }
      }
    })
  },

  onChangeLogins(newValue) {                
    let logins = parseLoginStr(newValue)            
    this.props.onChange({ ...this.props.ssh, logins });
  },

  onBlur(e) {    
    let { logins } = this.props.ssh;    
    let valueToAppend = parseLoginStr(e.target.value);
    logins = [...logins, ...valueToAppend]; 
    this.props.onChange({ ...this.props.ssh, logins });
  },

  onChangeDuration(e) {    
    let maxTtlText = e.target.value;    
    this.props.onChange({ ...this.props.ssh, maxTtlText });
  },

  onChangeLabels(e) {        
    let nodeLabelsText = e.target.value;
    this.props.onChange({ ...this.props.ssh, nodeLabelsText })
  },

  renderOption(option) {
    let text = defaultLoginDescription[option.value] || '';
    return (
      <div>
        {option.label}
        <div>
          <small className="text-muted">{text}</small>
        </div>
      </div>
    );
	},
    
  render() {    
    let { editable, ssh, onChange } = this.props;            
    let { enabled, logins, maxTtlText, nodeLabelsText} = ssh;
    return (                                        
      <Form className="m-t" refCb={e => this.refForm = e}>  
        <Layout.Flex dir="column" justify="between" style={{ flex: "1" }} align="center">
          <h3 className="no-margins"> Session Restriction</h3>
        </Layout.Flex>
        <Separator />        
        <div className="m-l">
          <Layout.Flex dir="row" justify="between" style={{ flex: "1", flexWrap: "wrap" }} align="baseline">
            <div className="checkbox m-r-sm">
              <label>
                <input
                  type="checkbox"
                  disabled={!editable}
                  checked={enabled}
                  onChange={() => onChange({ ...ssh, enabled: !enabled }) } />
                Allow SSH access
              </label>
            </div>            
          </Layout.Flex>
          <div className="help-block m-t-n-xs m-l-md">
            {DESC}
          </div>
        </div>
        {enabled ?
          <div>
            <Layout.Flex dir="row" className="m-l-lg m-t-md">            
              <Label>
                Max session duration:
              </Label>
              <div style={{ flex: '1', marginTop: "-6px" }}>
                <input                  
                  name="ttlDuration"
                  value={maxTtlText}
                  onChange={this.onChangeDuration}
                  className="form-control required grv-settings-roles-max-input-width"
                  placeholder="12h30m" />
                <div className="help-block m-t-sm">
                  {MAX_SESSION_DESC}
                </div>
              </div>
            </Layout.Flex>                            
            <Layout.Flex dir="row" className="m-l-lg m-t">
              <Label>
                Allowed Logins:
              </Label>
              <div style={{ flex: '1', marginTop: "-6px" }}>
                <Select                    
                  disabled={!editable}
                  className="grv-settings-label-selector-with-options grv-settings-roles-max-input-width"
                  classRules="required"
                  searchable multi simpleValue allowCreate clearable={false}
                  value={logins}                  
                  noResultsText="Enter a login name"
                  options={defaultLoginOptions}
                  optionRenderer={this.renderOption}                  
                  placeholder="Enter logins"
                  onBlur={this.onBlur}
                  onChange={this.onChangeLogins} />
                <div className="help-block m-t-sm">
                  {LOGIN_DESC}
                </div>
              </div>              
            </Layout.Flex>                        
            <Layout.Flex dir="row" className="m-l-lg m-t">
              <Label>
                Allow Nodes with Labels:
              </Label>
              <div style={{ flex: '1', marginTop: "-6px" }}>
                <input         
                  disabled={!editable}  
                  name="grvNodeLabels"
                  value={nodeLabelsText}
                  onChange={this.onChangeLabels} 
                  className="form-control required grv-settings-roles-max-input-width"
                  placeholder="access=interns,env=staging" />
                <div className="help-block m-t-sm">
                  {NODE_LABELS_DESC}
                </div>            
              </div>          
            </Layout.Flex>            
          </div> : null}
      </Form>            
    );
  }
});

export default RoleSshAccess;