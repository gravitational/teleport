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

// This code is taken from examples in react.js repository

// Simple pure-React component so we don't have to remember
// Bootstrap's classes
var BootstrapButton = React.createClass({
    render: function() {
        return (
            <a {...this.props}
               href="javascript:;"
               role="button"
               className={(this.props.className || '') + ' btn'} />
        );
    }
});

var BootstrapModal = React.createClass({
  // The following two methods are the only places we need to
  // integrate Bootstrap or jQuery with the components lifecycle methods.
    componentDidMount: function() {
        // When the component is added, turn it into a modal
        $(this.getDOMNode())
              .modal({backdrop: 'static', keyboard: false, show: false})
    },
    componentWillUnmount: function() {
        $(this.getDOMNode()).off('hidden', this.handleHidden);
    },
    close: function() {
        $(this.getDOMNode()).modal('hide');
    },
    open: function() {
        $(this.getDOMNode()).modal('show');
    },
    setConfirmText: function(text){
        $(React.findDOMNode(this.refs.confirm)).text(text);
    },
    render: function() {
        var confirmButton = null;
        var cancelButton = null;

        if (this.props.confirm) {
            confirmButton = (
                <BootstrapButton onClick={this.handleConfirm} className="btn btn-primary" ref="confirm">
                  {this.props.confirm}
                </BootstrapButton>
            );
        }
        if (this.props.cancel) {
            cancelButton = (
                <BootstrapButton onClick={this.handleCancel} className="btn btn-white" ref="cancel">
                  {this.props.cancel}
                </BootstrapButton>
            );
        }

        var className = "fa modal-icon " + (this.props.icon || "");
        var dialogClass = "modal-dialog " + (this.props.dialogClass || "")
            var inmodalClass = this.props.inmodal? "modal inmodal": "modal";
        return (
            <div className={inmodalClass} role="dialog" aria-hidden="true" id={this.props.id}>
              <div className={dialogClass}>
                <div className="modal-content">
                  <div className="modal-header">
                    <button type="button" className="close" onClick={this.handleCancel}>
                      <span aria-hidden="true">&times;</span><span className="sr-only">Close</span>
                    </button>
                    <h4 className="modal-title"><i className={className}></i> {this.props.title}</h4>
                    <small className="font-bold"> </small>
                  </div>
                  <div className="modal-body">
                    {this.props.children}
                  </div>
                  <div className="modal-footer">
                    {cancelButton}
                    {confirmButton}
                  </div>
                </div>
              </div>
            </div>
        );
    },
    handleCancel: function() {
        if (this.props.onCancel) {
            this.props.onCancel();
        }
    },
    handleConfirm: function() {
        if (this.props.onConfirm) {
            this.props.onConfirm();
        }
    }
});
