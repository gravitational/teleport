var React = require('react');
var NavLeftBar = require('./navLeftBar');
var reactor = require('app/reactor');
var {actions, getters} = require('app/modules/app');
var SelectNodeDialog = require('./selectNodeDialog.jsx');
var NotificationHost = require('./notificationHost.jsx');

var App = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      app: getters.appState
    }
  },

  componentWillMount(){
    actions.initApp();
    this.refreshInterval = setInterval(actions.fetchNodesAndSessions, 35000);
  },

  componentWillUnmount: function() {
    clearInterval(this.refreshInterval);
  },

  render: function() {
    if(this.state.app.isInitializing){
      return null;
    }

    return (
      <div className="grv-tlpt grv-flex grv-flex-row">
        <SelectNodeDialog/>
        <NotificationHost/>
        {this.props.CurrentSessionHost}
        <NavLeftBar/>
        {this.props.children}
      </div>
    );
  }
})

module.exports = App;
