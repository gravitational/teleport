var React = require('react');

var GoogleAuthInfo = React.createClass({
  render() {
    return (
      <div className="grv-google-auth">
        <div className="grv-google-auth-icon"></div>
        <strong>Google Authenticator</strong>
        <div>Download <a href="https://support.google.com/accounts/answer/1066447?hl=en">Google Authenticator</a> on your phone to access your two factory token</div>
      </div>
    );
  }
})

module.exports = GoogleAuthInfo;
