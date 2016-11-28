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

var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var {actions, getters} = require('app/modules/user');
var GoogleAuthInfo = require('./googleAuthLogo');
var cfg = require('app/config');
var {TeleportLogo} = require('./icons.jsx');
var {PROVIDER_GOOGLE, SECOND_FACTOR_TYPE_HOTP, SECOND_FACTOR_TYPE_OIDC, SECOND_FACTOR_TYPE_U2F} = require('app/services/auth');

var LoginInputForm = React.createClass({

  mixins: [LinkedStateMixin],

  getInitialState() {
    return {
      user: '',
      password: '',
      token: '',
      provider: null,
      secondFactorType: SECOND_FACTOR_TYPE_HOTP
    }
  },


  onLogin(e){
    e.preventDefault();
    this.state.secondFactorType = SECOND_FACTOR_TYPE_HOTP;
    // token field is required for Google Authenticator
    $('input[name=token]').addClass("required");
    if (this.isValid()) {
      this.props.onClick(this.state);
    }
  },

  onLoginWithGoogle: function(e) {
    e.preventDefault();
    this.state.secondFactorType = SECOND_FACTOR_TYPE_OIDC;
    this.state.provider = PROVIDER_GOOGLE;
    this.props.onClick(this.state);
  },

  onLoginWithU2f: function(e) {
    e.preventDefault();
    this.state.secondFactorType = SECOND_FACTOR_TYPE_U2F;
    // token field not required for U2F
    $('input[name=token]').removeClass("required");
    if (this.isValid()) {
      this.props.onClick(this.state);
    }
  },

  isValid: function() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  render() {
    let {isProcessing, isFailed, message } = this.props.attemp;
    let providers = cfg.getAuthProviders();
    let useGoogle = providers.indexOf(PROVIDER_GOOGLE) !== -1;
    let useU2f = !!cfg.getU2fAppId();

    return (
      <form ref="form" className="grv-login-input-form">
        <h3> Welcome to Teleport </h3>
        <div className="">
          <div className="form-group">
            <input autoFocus valueLink={this.linkState('user')} className="form-control required" placeholder="User name" name="userName" />
          </div>
          <div className="form-group">
            <input valueLink={this.linkState('password')} type="password" name="password" className="form-control required" placeholder="Password"/>
          </div>
          <div className="form-group">
            <input autoComplete="off" valueLink={this.linkState('token')} className="form-control required" name="token" placeholder="Two factor token (Google Authenticator)"/>
          </div>
          <button onClick={this.onLogin} disabled={isProcessing} type="submit" className="btn btn-primary block full-width m-b">Login</button>
          { useU2f ? <button onClick={this.onLoginWithU2f} disabled={isProcessing} type="submit" className="btn btn-primary block full-width m-b">Login with U2F</button> : null }
          { useGoogle ? <button onClick={this.onLoginWithGoogle} type="submit" className="btn btn-danger block full-width m-b">With Google</button> : null }
          { isProcessing && this.state.secondFactorType == SECOND_FACTOR_TYPE_U2F ? (<label className="help-block">Insert your U2F key and press the button on the key</label>) : null }
          { isFailed ? (<label className="error">{message}</label>) : null }
        </div>
      </form>
    );
  }
})

var Login = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      attemp: getters.loginAttemp
    }
  },

  onClick(inputData){
    var loc = this.props.location;
    var redirect = cfg.routes.app;

    if(loc.state && loc.state.redirectTo){
      redirect = loc.state.redirectTo;
    }

    actions.login(inputData, redirect);
  },

  render() {
    return (
      <div className="grv-login text-center">
        <TeleportLogo/>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">
            <LoginInputForm attemp={this.state.attemp} onClick={this.onClick}/>
            <GoogleAuthInfo/>
            <div className="grv-login-info">
              <i className="fa fa-question"></i>
              <strong>New Account or forgot password?</strong>
              <div>Ask for assistance from your Company administrator</div>
            </div>
          </div>
        </div>
      </div>
    );
  }
});

module.exports = Login;
