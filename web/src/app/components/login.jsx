var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var {actions, getters} = require('app/modules/user');
var GoogleAuthInfo = require('./googleAuthLogo');
var cfg = require('app/config');

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
      this.props.onClick(this.state);
    }
  },

  isValid: function() {
    var $form = $(this.refs.form);
    return $form.length === 0 || $form.valid();
  },

  render() {
    let {isProcessing, isFailed, message } = this.props.attemp;

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
            <input valueLink={this.linkState('token')} className="form-control required" name="token" placeholder="Two factor token (Google Authenticator)"/>
          </div>
          <button onClick={this.onClick} disabled={isProcessing} type="submit" className="btn btn-primary block full-width m-b">Login</button>
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
        <div className="grv-logo-tprt"></div>
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
