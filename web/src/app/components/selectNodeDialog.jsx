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
var reactor = require('app/reactor');
var {getters} = require('app/modules/dialogs');
var {closeSelectNodeDialog} = require('app/modules/dialogs/actions');
var NodeList = require('./nodes/nodeList.jsx');
var currentSessionGetters = require('app/modules/currentSession/getters');
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
    var activeSession = reactor.evaluate(currentSessionGetters.currentSession) || {};
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
