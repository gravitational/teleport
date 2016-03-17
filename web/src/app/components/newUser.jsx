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
var {actions, getters} = require('app/modules/invite');
var userModule = require('app/modules/user');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var GoogleAuthInfo = require('./googleAuthLogo');
var {ExpiredInvite} = require('./errorPage');

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
      token: ''
    }
  },

  onClick(e) {
    e.preventDefault();
    if (this.isValid()) {
      userModule.actions.signUp({
        name: this.state.name,
        psw: this.state.psw,
        token: this.state.token,
        inviteToken: this.props.invite.invite_token});
    }
  },

  isValid() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  render() {
    let {isProcessing, isFailed, message } = this.props.attemp;
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
          <div className="form-group">
            <input
              autoComplete="off"
              name="token"
              valueLink={this.linkState('token')}
              className="form-control required"
              placeholder="Two factor token (Google Authenticator)" />
          </div>
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

  render: function() {
    let {fetchingInvite, invite, attemp} = this.state;

    if(fetchingInvite.isFailed){
      return <ExpiredInvite/>
    }

    if(!invite) {
      return null;
    }

    return (
      <div className="grv-invite text-center">
        <div className="grv-logo-tprt"></div>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">
            <InviteInputForm attemp={attemp} invite={invite.toJS()}/>
            <GoogleAuthInfo/>
          </div>
          <div className="grv-flex-column grv-invite-barcode">
            <h4>Scan bar code for auth token <br/> <small>Scan below to generate your two factor token</small></h4>
            <img className="img-thumbnail" src={ `data:image/png;base64,${invite.get('qr')}` } />
          </div>
        </div>
      </div>
    );
  }
});

module.exports = Invite;
