var React = require('react');
var reactor = require('app/reactor');
var {getters} = require('app/modules/dialogs');
var {closeSelectNodeDialog} = require('app/modules/dialogs/actions');
var {changeServer} = require('app/modules/activeTerminal/actions');
var NodeList = require('./nodes/main.jsx');

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

  onLoginClick(serverId, login){
    if(SelectNodeDialog.onServerChangeCallBack){
      SelectNodeDialog.onServerChangeCallBack({serverId});
    }

    closeSelectNodeDialog();
  },

  componentWillUnmount(callback){
    $('.modal').modal('hide');
  },

  componentDidMount(){
    $('.modal').modal('show');
  },

  render() {
    return (
      <div className="modal fade grv-dialog-select-node" tabIndex={-1} role="dialog">
        <div className="modal-dialog">
          <div className="modal-content">
            <div className="modal-header">
            </div>
            <div className="modal-body">
              <NodeList onLoginClick={this.onLoginClick}/>
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
