var React = require('react');
var reactor = require('app/reactor');
var {getters} = require('app/modules/dialogs');
var {closeSelectNodeDialog} = require('app/modules/dialogs/actions');
var NodeList = require('./nodes/nodeList.jsx');
var activeSessionGetters = require('app/modules/activeTerminal/getters');
var nodeGetters = require('app/modules/nodes/getters');
var $ = require('jQuery');

var SelectNodeDialog = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      dialogs: getters.dialogs
    }
  },

  render() {
    return this.state.dialogs.isSelectNodeDialogOpen ? <Dialog/> : null;
  }
});

var Dialog = React.createClass({

  onLoginClick(serverId){
    if(SelectNodeDialog.onServerChangeCallBack){
      SelectNodeDialog.onServerChangeCallBack({serverId});
    }

    closeSelectNodeDialog();
  },

  componentWillUnmount(){
    $('.modal').modal('hide');
  },

  componentDidMount(){
    $('.modal').modal('show');
  },

  render() {
    var activeSession = reactor.evaluate(activeSessionGetters.activeSession) || {};
    var nodeRecords = reactor.evaluate(nodeGetters.nodeListView);
    var logins = [activeSession.login];

    return (
      <div className="modal fade grv-dialog-select-node" tabIndex={-1} role="dialog">
        <div className="modal-dialog">
          <div className="modal-content">
            <div className="modal-header">
            </div>
            <div className="modal-body">
              <NodeList nodeRecords={nodeRecords} logins={logins} onLoginClick={this.onLoginClick}/>
            </div>
            <div className="modal-footer">
              <button onClick={closeSelectNodeDialog} type="button" className="btn btn-primary">
                Close
              </button>
            </div>
          </div>
        </div>
      </div>
    );
  }
});

SelectNodeDialog.onServerChangeCallBack = ()=>{};

module.exports = SelectNodeDialog;
