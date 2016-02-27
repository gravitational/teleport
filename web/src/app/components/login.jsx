var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var {actions} = require('app/modules/user');
var GoogleAuthInfo = require('./googleAuth');
var LoginInputForm = React.createClass({

  mixins: [LinkedStateMixin],

  getInitialState() {
    return {
      user: '',
      password: '',
      token: ''
    }
  },

  onClick: function(e) {
    e.preventDefault();
    if (this.isValid()) {
      actions.login({ ...this.state}, '/web');
    }
  },

  isValid: function() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  render() {
    return (
      <form ref="form" className="grv-login-input-form">
        <h3> Welcome to Teleport </h3>
        <div className="">
          <div className="form-group">
            <input valueLink={this.linkState('user')} className="form-control required" placeholder="User name" name="userName" />
          </div>
          <div className="form-group">
            <input valueLink={this.linkState('password')} type="password" name="password" className="form-control required" placeholder="Password"/>
          </div>
          <div className="form-group">
            <input valueLink={this.linkState('token')} className="form-control required" name="token" placeholder="Two factor token (Google Authenticator)"/>
          </div>
          <button type="submit" className="btn btn-primary block full-width m-b" onClick={this.onClick}>Login</button>
        </div>
      </form>
    );
  }
})

var Login = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
  //    userRequest: getters.userRequest
    }
  },

  render: function() {
    var isProcessing = false;//this.state.userRequest.get('isLoading');
    var isError = false;//this.state.userRequest.get('isError');

    return (
      <div className="grv-login text-center">
        <div className="grv-logo-tprt"></div>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">
            <LoginInputForm/>
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
