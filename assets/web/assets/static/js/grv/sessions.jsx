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
'use strict';

var SessionsPage = React.createClass({
  getInitialState: function(){
      return {sessions: []};
  },
  componentDidMount: function() {
      this.reload();
      setInterval(this.reload, this.props.pollInterval);
  },
  reload: function(){
    $.ajax({
      url: this.props.url,
      dataType: 'json',
      success: function(data) {
        if(this.modalOpen == true) {
           return
        }
        this.setState({sessions: data});
      }.bind(this),
      error: function(xhr, status, err) {
        console.error(this.props.url, status, err.toString());
      }.bind(this)
    });
  },
    render: function() {
    return (
        <div id="wrapper">
          <LeftNavBar current="sessions"/>
          <div id="page-wrapper" className="gray-bg">
            <TopNavBar/>
            <PageHeader title="Active Sessions" icon="fa fa-wechat"/>
            <div className="wrapper wrapper-content">
              <Box>
                <SessionsTable sessions={this.state.sessions}/>
              </Box>
            </div>
            <PageFooter/>
          </div>
        </div>
    );
  }
});


var SessionsTable = React.createClass({
  render: function() {
    var onConnect = this.props.onConnect
    var onDisconnect = this.props.onDisconnect
      var rows = this.props.sessions.map(function (se, index) {
          return (
              <SessionRow se={se} key={index}/>
          );
      });
      return (
          <table className="table table-striped">
            <thead>
              <tr>
                <th>ID</th>
                <th>LastActive</th>
                <th>Connected users</th>
                <th>Connect</th>
              </tr>
            </thead>
            <tbody>
              {rows}
            </tbody>
          </table>);
  }
});

var SessionRow = React.createClass({
    render: function() {
        var se = parseSession(this.props.se);
        var users = se.users.map(function (u, index) {
            return (
                <a href={grv.path("sessions", se.id)}>{u}&nbsp;</a>
            );
        });
        return (
            <tr>
              <td><a href={grv.path("sessions", se.id)}>{se.id}</a></td>
              <td>{timeSince(se.last_active)} ago</td>
              <td>{users}</td>              
              <td><a href={grv.path("sessions", se.id)}><i className="fa fa-tty text-navy"></i></a></td>
            </tr>
        );
  }
});

React.render(
  <SessionsPage url={grv.path("api", "sessions")} pollInterval={1000}/>,
  document.body
);
