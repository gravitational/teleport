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

import React from 'react';
import $ from 'jQuery';
import classnames from 'classnames';
import { connect } from 'nuclear-js-react-addons';
import cfg from 'app/config';
import actions from 'app/flux/user/actions';
import getters from 'app/flux/user/getters';
import { Auth2faTypeEnum } from 'app/services/enums';
import { ErrorPage, ErrorTypes } from './../msgPage';
import { TeleportLogo } from './../icons.jsx';
import GoogleAuthInfo from './googleAuthLogo';
import { ErrorMessage } from './items';

const U2F_HELP_URL = 'https://support.google.com/accounts/answer/6103523?hl=en';

const needs2fa = auth2faType => !!auth2faType && auth2faType !== Auth2faTypeEnum.DISABLED;

export class Invite extends React.Component {

  componentDidMount(){
    actions.fetchInvite(this.props.params.inviteToken);
  }
      
  onSubmitWithU2f = (username, password) => {    
    actions.acceptInviteWithU2f(username, password, this.props.params.inviteToken);
  }

  onSubmit = (username, password, token) => {    
    actions.acceptInvite(username, password, token, this.props.params.inviteToken);
  }

  render() {
    let {fetchingInvite, invite, attemp} = this.props;                
    let auth2faType = cfg.getAuth2faType();
            
    if(fetchingInvite.isFailed){
      return <ErrorPage type={ErrorTypes.EXPIRED_INVITE}/>
    }

    if(!invite) {
      return null;
    }
    
    let containerClass = classnames('grv-invite-content grv-flex', {
      '---with-2fa-data': needs2fa(auth2faType)
    })
    
    return (
      <div className="grv-invite text-center">
        <TeleportLogo />                
        <div className={containerClass}>
          <div className="grv-flex-column">
            <InviteInputForm              
              auth2faType={auth2faType}              
              attemp={attemp}
              invite={invite}              
              onSubmitWithU2f={this.onSubmitWithU2f}
              onSubmit={this.onSubmit}                          
            />
            <InviteFooter auth2faType={auth2faType}/>
          </div>
          <Invite2faData
            auth2faType={auth2faType}
            qr={invite.qr} />          
        </div>        
      </div>
    );
  }
}

export class InviteInputForm extends React.Component {

  static propTypes = {    
    auth2faType: React.PropTypes.string,
    authType: React.PropTypes.string,  
    onSubmitWithU2f: React.PropTypes.func.isRequired,
    onSubmit: React.PropTypes.func.isRequired,
    attemp: React.PropTypes.object.isRequired
  }

  constructor(props) {
    super(props);
    this.state = {
      userName: this.props.invite.user,
      password: '',
      passwordConfirmed: '',
      token: ''  
    }
  }
  
  componentDidMount(){
    $(this.refs.form).validate({
      rules:{
        password:{
          minlength: 6,
          required: true
        },
        passwordConfirmed:{
          required: true,
          equalTo: this.refs.password
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
  
  onSubmit = e => {    
    e.preventDefault();    
    if (this.isValid()) {
      let { userName, password, token } = this.state;
      this.props.onSubmit(userName, password, token);
    }
  }

  onSubmitWithU2f = e => {    
    e.preventDefault();    
    if (this.isValid()) {
      let { userName, password } = this.state;
      this.props.onSubmitWithU2f(userName, password);
    }
  }

  onChangeState = (propName, value) => {
    this.setState({
      [propName]: value
    });
  }
        
  isValid() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  }
      
  renderNameAndPassFields() {    
    return (
      <div>
        <div className="form-group">
          <input
            disabled  
            autoFocus                        
            value={this.state.userName}
            onChange={e => this.onChangeState('userName', e.target.value)}
            className="form-control required"
            placeholder="User name"
            name="userName"/>
        </div>
        <div className="form-group">
          <input
            value={this.state.password}
            onChange={e => this.onChangeState('password', e.target.value)}
            ref="password"
            type="password"
            name="password"
            className="form-control required"
            placeholder="Password"/>
        </div>
        <div className="form-group">
          <input            
            value={this.state.passwordConfirmed}
            onChange={e => this.onChangeState('passwordConfirmed', e.target.value)}
            type="password"
            name="passwordConfirmed"
            className="form-control"
            placeholder="Password confirm"/>
          </div>          
      </div>
    )
  }

  render2faFields() {
    let { auth2faType } = this.props;
    if (needs2fa(auth2faType) && auth2faType === Auth2faTypeEnum.OTP) {
      return (
      <div className="form-group">
        <input
          autoComplete="off"
          value={this.state.token}
          onChange={e => this.onChangeState('token', e.target.value)}
          className="form-control required"
          name="token"
          placeholder="Two factor token (Google Authenticator)"/>
      </div>
      )
    }

    return null;            
  }

  renderSubmitBtn() {    
    let { isProcessing } = this.props.attemp;        
    let $helpBlock = isProcessing && this.props.auth2faType === Auth2faTypeEnum.UTF ? (
      <div className="help-block">
        Insert your U2F key and press the button on the key
        </div>
    ) : null;


    let onClick = this.props.auth2faType === Auth2faTypeEnum.UTF ?
      this.onSubmitWithU2f : this.onSubmit;

    return (
      <div>
        <button
          onClick={onClick}
          disabled={isProcessing}
          type="submit"
          className="btn btn-primary block full-width m-b">
          Sign up
        </button>
        {$helpBlock}        
      </div>
    );        
  }
      
  render() {            
    let { isFailed, message } = this.props.attemp;        
    let $error = isFailed ? <ErrorMessage message={message} /> : null;
    return (
      <form ref="form" className="grv-invite-input-form">
        <h3> Get started with Teleport </h3>
        {this.renderNameAndPassFields()}    
        {this.render2faFields()}
        {this.renderSubmitBtn()}                                  
        {$error}
      </form>
    );
  }
}

const Invite2faData = ({auth2faType, qr}) => {
  if (!needs2fa(auth2faType)) {
    return null;
  }
  
  if (auth2faType === Auth2faTypeEnum.OTP) {
    return (
      <div className="grv-flex-column grv-invite-barcode">
        <h4>Scan bar code for auth token <br />
          <small>Scan below to generate your two factor token</small>
        </h4>
        <img className="img-thumbnail" src={ `data:image/png;base64,${qr}` } />
      </div>
    )
  }

  if (auth2faType === Auth2faTypeEnum.UTF) {
    return (
      <div className="grv-flex-column">
        <h3>Insert your U2F key </h3>
        <div className="m-t-md">Press the button on the U2F key after you press the sign up button</div>        

        <div className="m-t text-muted">
          <small>Click
            <a a target="_blank" href={U2F_HELP_URL}> here </a>
            to learn more about U2F 2-Step Verification.
          </small>
        </div>

    </div>
    )
  }

  return null;
}

const InviteFooter = ({auth2faType}) => {
  let $googleHint = auth2faType === Auth2faTypeEnum.OTP ? <GoogleAuthInfo /> : null;
  return (
    <div>
      {$googleHint}      
    </div>
  )
}

function mapStateToProps() {
  return {
      invite: getters.invite,
      attemp: getters.attemp,
      fetchingInvite: getters.fetchingInvite
    }  
}

export default connect(mapStateToProps)(Invite);
