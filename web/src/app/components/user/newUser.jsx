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
var {actions, getters} = require('app/modules/user');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var GoogleAuthInfo = require('./googleAuthLogo');
var {ErrorPage, ErrorTypes} = require('./../msgPage');
var {TeleportLogo} = require('./../icons.jsx');
var {SECOND_FACTOR_TYPE_HOTP, SECOND_FACTOR_TYPE_U2F} = require('app/services/auth');
var cfg = require('app/config');

var InviteInputForm = React.createClass({

  mixins: [LinkedStateMixin],

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
  },

  getInitialState() {
    return {
      name: this.props.invite.user,
      psw: '',
      pswConfirmed: '',
      token: '',
      secondFactorType: SECOND_FACTOR_TYPE_HOTP
    }
  },

  onClick(e) {
    e.preventDefault();
    if (this.isValid()) {
      actions.signUp({
        name: this.state.name,
        psw: this.state.psw,
        token: this.state.token,
        inviteToken: this.props.invite.invite_token,
        secondFactorType: this.state.secondFactorType});
    }
  },

  isValid() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  onSecondFactorTypeChanged(e){
    var type = e.currentTarget.value;
    this.setState({
      secondFactorType: type
    });
    this.props.set2FType(type);
  },

  render() {
    let {isProcessing, isFailed, message } = this.props.attemp;
    let useU2f = !!cfg.getU2fAppId();
    return (
      <form ref="form" className="grv-invite-input-form">
        <h3> Get started with Teleport </h3>
        <div className="">
          <div className="form-group">
            <input
              disabled
              valueLink={this.linkState('name')}
              name="userName"
              className="form-control required"
              placeholder="User name"/>
          </div>
          <div className="form-group">
            <input
              autoFocus
              valueLink={this.linkState('psw')}
              ref="password"
              type="password"
              name="password"
              className="form-control"
              placeholder="Password" />
          </div>
          <div className="form-group">
            <input
              valueLink={this.linkState('pswConfirmed')}
              type="password"
              name="passwordConfirmed"
              className="form-control"
              placeholder="Password confirm"/>
          </div>
          { useU2f ?
          <div className="form-group">
            <input
              type="radio"
              value={ SECOND_FACTOR_TYPE_HOTP }
              checked={ this.state.secondFactorType == SECOND_FACTOR_TYPE_HOTP }
              onChange={ this.onSecondFactorTypeChanged }/>
            Google Authenticator
            <input
              type="radio"
              value={ SECOND_FACTOR_TYPE_U2F }
              checked={ this.state.secondFactorType == SECOND_FACTOR_TYPE_U2F }
              onChange={ this.onSecondFactorTypeChanged }/>
            U2F
          </div>
          : null }
          { this.state.secondFactorType == SECOND_FACTOR_TYPE_HOTP ?
          <div className="form-group">
            <input
              autoComplete="off"
              name="token"
              valueLink={this.linkState('token')}
              className="form-control required"
              placeholder="Two factor token (Google Authenticator)" />
          </div>
          : null }
          <button type="submit" disabled={isProcessing} className="btn btn-primary block full-width m-b" onClick={this.onClick} >Sign up</button>
          { isFailed ? (<label className="error">{message}</label>) : null }
        </div>
      </form>
    );
  }
})

var Invite = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      invite: getters.invite,
      attemp: getters.attemp,
      fetchingInvite: getters.fetchingInvite
    }
  },

  componentDidMount(){
    actions.fetchInvite(this.props.params.inviteToken);
  },

  getInitialState() {
    return {
      secondFactorType: SECOND_FACTOR_TYPE_HOTP
    }
  },

  setSecondFactorType(type){
    this.setState({
      secondFactorType: type
    });
  },

  render: function() {
    let {fetchingInvite, invite, attemp} = this.state;

    if(fetchingInvite.isFailed){
      return <ErrorPage type={ErrorTypes.EXPIRED_INVITE}/>
    }

    if(!invite) {
      return null;
    }

    return (
      <div className="grv-invite text-center">
        <TeleportLogo/>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">
            <InviteInputForm attemp={attemp} invite={invite.toJS()} set2FType={this.setSecondFactorType}/>
            <GoogleAuthInfo/>
          </div>
          { this.state.secondFactorType == SECOND_FACTOR_TYPE_HOTP ?
          <div className="grv-flex-column grv-invite-barcode">
            <h4>Scan bar code for auth token <br/> <small>Scan below to generate your two factor token</small></h4>
            <img className="img-thumbnail" src={ `data:image/png;base64,${invite.get('qr')}` } />
          </div>
          :
          <div className="grv-flex-column">
            <h4>Insert your U2F key <br/> <small>Press the button on the U2F key after you press the sign up button</small></h4>
          </div>
          }
        </div>
      </div>
    );
  }
});

module.exports = Invite;
