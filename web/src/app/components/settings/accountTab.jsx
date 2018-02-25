/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import $ from 'jQuery';
import React from 'react';
import connect from 'app/components/connect';
import cfg from 'app/config';
import { Auth2faTypeEnum } from 'app/services//enums';
import * as Alerts from 'app/components/alerts';

import { getters } from 'app/flux/user';
import * as actions from 'app/flux/settingsAccount/actions';
import Layout from 'app/components/layout';

const Separator = () => <div className="grv-settings-header-line-solid m-t-sm m-b-sm"/>;

const Label = ({text}) => ( 
  <label style={{ width: "150px", fontWeight: "normal" }} className=" m-t-xs"> {text} </label>
)

const defaultState = {
  oldPass: '',
  newPass: '',
  newPassConfirmed: '',
  token: ''
}

class AccountTab extends React.Component {

  static propTypes = {
    attempt: React.PropTypes.object.isRequired,
    onChangePass: React.PropTypes.func.isRequired,
    onChangePassWithU2f: React.PropTypes.func.isRequired
  }
  
  hasBeenClicked = false;

  state = { ...defaultState };
  
  componentDidMount() {
    $(this.refs.form).validate({
      rules: {
        newPass: {
          minlength: 6,
          required: true
        },
        newPassConfirmed: {
          required: true,
          equalTo: this.refs.newPass
        }
      },
      messages: {
        passwordConfirmed: {
          minlength: $.validator.format('Enter at least {0} characters'),
          equalTo: 'Enter the same password as above'
        }
      }
    })
  }

  componentWillUnmount() {
    this.props.onDestory && this.props.onDestory();
  }
  
  onClick = e => {
    e.preventDefault();
    if (this.isValid()) {
      const { oldPass, newPass, token } = this.state;      
      this.hasBeenClicked = true;
      if (this.props.auth2faType === Auth2faTypeEnum.UTF) {
        this.props.onChangePassWithU2f(oldPass, newPass);
      } else {
        this.props.onChangePass(oldPass, newPass, token);
      }                  
    }
  }
  
  onKeyPress = e => {        
    if (e.key === 'Enter' && e.target.value) {              
      this.onClick(e)
    }        
  }

  isValid() {
    const $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  }

  componentWillReceiveProps(nextProps) {
    const { isSuccess } = nextProps.attempt;    
    if (isSuccess && this.hasBeenClicked) {
      // reset all input fields on success
      this.hasBeenClicked = false;
      this.setState(defaultState);
    }
  }

  isU2f() {
    return this.props.auth2faType === Auth2faTypeEnum.UTF;
  }

  isOtp() {
    return this.props.auth2faType === Auth2faTypeEnum.OTP;
  }

  render() {
    const isOtpEnabled = this.isOtp();  
    const { isFailed, isProcessing, isSuccess, message } = this.props.attempt;    
    const { oldPass, newPass, newPassConfirmed } = this.state;
    const waitForU2fKeyResponse = isProcessing && this.isU2f()
    
    return (
      <div title="Change Password" className="m-t-sm grv-settings-account">
        <h3 className="no-margins">Change Password</h3>
        <Separator />        
        <div className="m-b m-l-xl" style={{ maxWidth: "500px" }}>          
          <form ref="form" onKeyPress={this.onKeyPress}>
            <div>          
              { isFailed && <Alerts.Danger className="m-b-sm"> {message} </Alerts.Danger> }
              { isSuccess && <Alerts.Success className="m-b-sm"> Your password has been changed </Alerts.Success> }
              { waitForU2fKeyResponse && <Alerts.Info className="m-b-sm"> Insert your U2F key and press the button on the key </Alerts.Info> }
            </div>                                 
            <Layout.Flex dir="row" className="m-t">
              <Label text="Current Password:" />
              <div style={{ flex: "1" }}>
                <input
                  autoFocus
                  type="password"
                  value={oldPass}                  
                  onChange={e => this.setState({
                    oldPass: e.target.value
                  })}
                  className="form-control required"/>
              </div>
            </Layout.Flex>
            {isOtpEnabled &&
              <Layout.Flex dir="row" className="m-t-sm">
                <Label text="2nd factor token:" />
                <div style={{ flex: "1" }}>
                  <input autoComplete="off"
                    style={{width: "100px"}}  
                    value={this.state.token}
                    onChange={e => this.setState({
                      'token': e.target.value
                    })}
                    className="form-control required" name="token"
                  />
                </div>
              </Layout.Flex>
            }  
            <Layout.Flex dir="row" className="m-t-lg">
              <Label text="New Password:" />
              <div style={{ flex: "1" }}>
                <input
                  value={newPass}
                  onChange={e => this.setState({
                    newPass: e.target.value
                  })}
                  ref="newPass"
                  type="password"
                  name="newPass"
                  className="form-control"
                />
              </div>
            </Layout.Flex>
            <Layout.Flex dir="row" className="m-t-sm">
              <Label text="Confirm Password:" />
              <div style={{ flex: "1" }}>
                <input
                  type="password"
                  value={newPassConfirmed}
                  onChange={e => this.setState({
                    newPassConfirmed: e.target.value
                  })}
                  name="newPassConfirmed"
                  className="form-control"
                />                
              </div>
            </Layout.Flex>            
          </form>
        </div>
        <button disabled={isProcessing} onClick={this.onClick} type="submit" className="btn btn-sm btn-primary block" >Update</button>
      </div>
    )
  }
}
  
function mapFluxToProps() {
  return {        
    attempt: getters.pswChangeAttempt
  }
}

function mapStateToProps() {
  return {
    auth2faType: cfg.getAuth2faType(),
    onChangePass: actions.changePassword,
    onChangePassWithU2f: actions.changePasswordWithU2f,
    onDestory: actions.resetPasswordChangeAttempt
  }
}

export default connect(mapFluxToProps, mapStateToProps)(AccountTab);
