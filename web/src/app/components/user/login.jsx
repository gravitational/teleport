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
import 'app/../assets/js/jquery-validate';
import reactor from 'app/reactor';
import {actions, getters} from 'app/modules/user';
import GoogleAuthInfo from './googleAuthLogo';
import cfg from 'app/config';
import { TeleportLogo } from './../icons.jsx';
import { SsoBtnList } from './ssoBtnList';
import { Auth2faTypeEnum, AuthTypeEnum } from 'app/services/enums';

const Login = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      attemp: getters.loginAttemp
    }
  },

  onLoginWithOidc(providerName){
    let redirect = this.getRedirectUrl();            
    actions.loginWithOidc(providerName, redirect);
  },

  onLoginWithU2f(username, password) {
    let redirect = this.getRedirectUrl();            
    actions.loginWithU2f(username, password, redirect);
  },

  onLogin(username, password, token) {
    let redirect = this.getRedirectUrl();            
    actions.login(username, password, token, redirect);
  },

  getRedirectUrl() {
    let loc = this.props.location;
    let redirect = cfg.routes.app;

    if (loc.state && loc.state.redirectTo) {
      redirect = loc.state.redirectTo;
    }

    return redirect;    
  },

  render() {  
    let {attemp} = this.state;
    let authProviders = cfg.getAuthProviders();
    let authType = cfg.getAuthType();
    let auth2faType = cfg.getAuth2faType();
        
    return (
      <div className="grv-login text-center">
        <TeleportLogo/>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">            
            <LoginInputForm
              authProviders={authProviders}  
              auth2faType={auth2faType}
              authType={authType}              
              onLoginWithOidc={this.onLoginWithOidc}
              onLoginWithU2f={this.onLoginWithU2f}
              onLogin={this.onLogin}              
              attemp={attemp}
            />                            
            <LoginFooter auth2faType={auth2faType}/>
          </div>
        </div>
      </div>
    );
  }
});

const LoginInputForm = React.createClass({
  
  getInitialState() {    
    return {      
      user: '',
      password: '',
      token: ''      
    }
  },

  onLogin(e) {    
    e.preventDefault();    
    if (this.isValid()) {
      let { user, password, token } = this.state;
      this.props.onLogin(user, password, token);
    }
  },

  onLoginWithU2f(e) {    
    e.preventDefault();    
    if (this.isValid()) {
      let { user, password } = this.state;
      this.props.onLoginWithU2f(user, password);
    }
  },
      
  onLoginWithOidc(providerName) {    
    this.props.onLoginWithOidc(providerName);
  },

  isValid() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  needsCredentials() {
    return this.props.authType === AuthTypeEnum.LOCAL || this.needs2fa();
  },

  needs2fa() {
    return !!this.props.auth2faType && this.props.auth2faType !== Auth2faTypeEnum.DISABLED;
  },

  render2faFields() {
    if (!this.needs2fa() || this.props.auth2faType !== Auth2faTypeEnum.OTP) {
      return null;
    }
        
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
  },

  onChangeState(propName, value) {
    this.setState({
      [propName]: value
    });
  },

  renderNameAndPassFields() {
    if (!this.needsCredentials()) {
      return null;
    }

    return (
      <div>
        <div className="form-group">
          <input
            autoFocus            
            value={this.state.user}
            onChange={e => this.onChangeState('user', e.target.value)}
            className="form-control required"
            placeholder="User name"
            name="userName"/>
        </div>
        <div className="form-group">
          <input
            value={this.state.password}
            onChange={e => this.onChangeState('password', e.target.value)}
            type="password"
            name="password"
            className="form-control required"
            placeholder="Password"/>
        </div>
      </div>
    )
  },

  renderLoginBtn() {    
    let { isProcessing } = this.props.attemp;    
    if (!this.needsCredentials()) {
      return null;
    }

    let $helpBlock = isProcessing && this.props.auth2faType === Auth2faTypeEnum.UTF ? (
      <div className="help-block">
        Insert your U2F key and press the button on the key
        </div>
    ) : null;


    let onClick = this.props.auth2faType === Auth2faTypeEnum.UTF ?
      this.onLoginWithU2f : this.onLogin;

    return (
      <div>
        <button
          onClick={onClick}
          disabled={isProcessing}
          type="submit"
          className="btn btn-primary block full-width m-b">
          Login
        </button>
        {$helpBlock}        
      </div>
    );        
  },

  renderSsoBtns() {    
    let { authType, authProviders, attemp } = this.props;

    if (authType !== AuthTypeEnum.OIDC) {
      return null;
    }
        
    return (
      <SsoBtnList
        prefixText="Login with "
        isDisabled={attemp.isProcessing}
        providers={authProviders}
        onClick={this.onLoginWithOidc} />
    )    
  },

  render() {
    let { isFailed, message } = this.props.attemp;                    
    let $error = isFailed ? (
      <label className="error">{message}</label>
    ) : null;

    let hasAnyAuth = !!cfg.auth;
    
    return (
      <div>
        <form ref="form" className="grv-login-input-form">
          <h3> Welcome to Teleport </h3>
          {!hasAnyAuth ? <div> You have no authentication options configured </div>
            :
            <div>
              {this.renderNameAndPassFields()}
              {this.render2faFields()}
              {this.renderLoginBtn()}
              {this.renderSsoBtns()}
              {$error}
            </div>
          }
        </form>        
      </div>
    );
  }
})

LoginInputForm.propTypes = {  
  authProviders: React.PropTypes.array,
  auth2faType: React.PropTypes.string,
  authType: React.PropTypes.string,
  onLoginWithOidc: React.PropTypes.func.isRequired,
  onLoginWithU2f: React.PropTypes.func.isRequired,
  onLogin: React.PropTypes.func.isRequired,
  attemp: React.PropTypes.object.isRequired
}

const LoginFooter = ({auth2faType}) => {
  let $googleHint = auth2faType === Auth2faTypeEnum.OTP ? <GoogleAuthInfo /> : null;
  return (
    <div>
      {$googleHint}
      <div className="grv-login-info">
        <i className="fa fa-question"></i>
        <strong>New Account or forgot password?</strong>
        <div>Ask for assistance from your Company administrator</div>
      </div>
    </div>
  )
}

export default Login;
export {
  LoginInputForm
}