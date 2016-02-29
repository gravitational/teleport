var React = require('react');
var NavLeftBar = require('./navLeftBar');
var cfg = require('app/config');
var actions = require('app/modules/actions');
var {ActiveSession} = require('./activeSession/main.jsx');

var App = React.createClass({

  componentDidMount(){
    actions.fetchNodesAndSessions();
  },

  render: function() {
    return (
      <div className="grv-tlpt">
        <NavLeftBar/>
        <ActiveSession/>
        <div className="row">
          <nav className="" role="navigation" style={{ marginBottom: 0 }}>
            <ul className="nav navbar-top-links navbar-right">
              <li>
                <a href={cfg.routes.logout}>
                  <i className="fa fa-sign-out"></i>
                  Log out
                </a>
              </li>
            </ul>
          </nav>
        </div>
        <div className="grv-page">
          {this.props.children}
        </div>
      </div>
    );
  }
})

module.exports = App;
