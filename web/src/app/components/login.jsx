var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');
var LinkedStateMixin = require('react-addons-linked-state-mixin');
var {actions} = require('app/modules/user');

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
    //if (this.isValid()) {
      actions.login({ ...this.state}, '/web');
    //}
  },

  isValid: function() {
    var $form = $(".loginscreen form");
    return $form.length === 0 || $form.valid();
  },

  render() {
    return (
      <div>
        <h3> Welcome to Teleport </h3>
        <div className="">
          <div className="form-group">
            <input className="form-control" placeholder="Username" valueLink={this.linkState('user')}/>
          </div>
          <div className="form-group">
            <input type="password" className="form-control" placeholder="Password" valueLink={this.linkState('password')}/>
          </div>
          <div className="form-group">
            <input className="form-control" placeholder="Two factor token (Google Authenticator)"  valueLink={this.linkState('token')}/>
          </div>
          <button type="submit" className="btn btn-primary block full-width m-b" onClick={this.onClick}>Login</button>
        </div>
      </div>
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
      <div className="grv grv-login text-center">
        <div className="grv-logo-tprt"></div>
        <div className="grv-content grv-flex">
          <div className="grv-flex-column">
            <LoginInputForm/>
          </div>
        </div>
      </div>
    );
  }
});

module.exports = Login;
