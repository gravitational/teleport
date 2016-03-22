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
//var {TeleportLogo} = require('./icons.jsx');

var NotFound = React.createClass({
  render() {
    return (
      <div className="grv-error-page">

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
