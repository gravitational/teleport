var React = require('react');
var NavLeftBar = require('./navLeftBar');
var cfg = require('app/config');
var reactor = require('app/reactor');
var {actions, getters} = require('app/modules/app');
var SelectNodeDialog = require('./selectNodeDialog.jsx');

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
        {this.props.CurrentSessionHost}
        <NavLeftBar/>
        {this.props.children}
      </div>
    );
  }
})

module.exports = App;
