var React = require('react');
var $ = require('jQuery');
var reactor = require('app/reactor');

var Invite = React.createClass({

  handleSubmit: function(e) {
    e.preventDefault();
    if (this.isValid()) {
      var loc = this.props.location;
      var email = this.refs.email.value;
      var pass = this.refs.pass.value;
      var redirect = '/web';

      if (loc.state && loc.state.redirectTo) {
        redirect = loc.state.redirectTo;
      }

      //actions.login(email, pass, redirect);
    }
  },

  isValid: function() {
    var $form = $(".loginscreen form");
    return $form.length === 0 || $form.valid();
  },

  render: function() {
    var isProcessing = false; //this.state.userRequest.get('isLoading');
    var isError = false; //this.state.userRequest.get('isError');

    return (
      <div className="middle-box text-center loginscreen  animated fadeInDown">
        <div>
          <div>
            <h1 className="logo-name">G</h1>
          </div>
          <h3>Welcome to Gravity</h3>
          <p>Create password.</p>
          <div align="left">
            <font color="white">
              1) Create and enter a new password
              <br></br>
              2) Install Google Authenticator on your smartphone
              <br></br>
              3) Open Google Authenticator and create a new account using provided barcode
              <br></br>
              4) Generate Authenticator token and enter it below
              <br></br>
            </font>
          </div>
          <form className="m-t" role="form" action="/web/finishnewuser" method="POST">
            <div className="form-group">
              <input type="hidden" name="token" className="form-control"/>
            </div>
            <div className="form-group">
              <input type="test" name="username" disabled className="form-control" placeholder="Username" required=""/>
            </div>
            <div className="form-group">
              <input type="password" name="password" id="password" className="form-control" placeholder="Password" required="" onchange="checkPasswords()"/>
            </div>
            <div className="form-group">
              <input type="password" name="password_confirm" id="password_confirm" className="form-control" placeholder="Confirm password" required="" onchange="checkPasswords()"/>
            </div>
            <div className="form-group">
              <input type="test" name="hotp_token" id="hotp_token" className="form-control" placeholder="hotp token" required=""/>
            </div>
            <button type="submit" className="btn btn-primary block full-width m-b">Confirm</button>
          </form>
        </div>
      </div>
    );
  }
});

module.exports = Invite;
