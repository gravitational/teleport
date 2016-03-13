var React = require('react');

var NotFound = React.createClass({
  render() {
    return (
      <div className="grv-error-page">
        <div className="grv-logo-tprt">Teleport</div>
        <div className="grv-warning"><i className="fa fa-warning"></i> </div>
        <h1>Whoops, we cannot find that</h1>
        <div>Looks like the page you are looking for isn't here any longer</div>
        <div>If you believe this is an error, please contact your organization administrator.</div>
        <div className="contact-section">If you believe this is an issue with Teleport, please <a href="https://github.com/gravitational/teleport/issues/new">create a GitHub issue.</a>
         </div>
      </div>
    );
  }
})

var ExpiredInvite = React.createClass({
  render() {
    return (
      <div className="grv-error-page">
        <div className="grv-logo-tprt">Teleport</div>
        <div className="grv-warning"><i className="fa fa-warning"></i> </div>
        <h1>Invite code has expired</h1>
        <div>Looks like your invite code isn't valid anymore</div>
        <div className="contact-section">If you believe this is an issue with Teleport, please <a href="https://github.com/gravitational/teleport/issues/new">create a GitHub issue.</a>
         </div>
      </div>
    );
  }
})

export default NotFound;
export {NotFound, ExpiredInvite}
