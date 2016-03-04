var React = require('react');
var NavLeftBar = require('./navLeftBar');
var cfg = require('app/config');
var reactor = require('app/reactor');
var {actions, getters} = require('app/modules/app');

var App = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      app: getters.appState
    }
  },

  componentWillMount(){
    actions.initApp();
    this.refreshInterval = setInterval(actions.fetchNodesAndSessions, 3000);
  },

  componentWillUnmount: function() {
    clearInterval(this.refreshInterval);
  },

  render: function() {
    if(this.state.app.isInitializing){
      return null;
    }

    return (
      <div className="grv-tlpt">
        <NavLeftBar/>
        {this.props.activeSessionHost}
        <div className="row">
          <nav className="" role="navigation" style={{ marginBottom: 0, float: "right" }}>
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
