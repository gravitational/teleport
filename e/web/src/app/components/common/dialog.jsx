import React from 'react';
import $ from 'jQuery';

const GrvDialogDefaultHeader = ({title}) => <h2 className="m-t-xs">{title}</h2>;

const GrvDialogHeader = React.createClass({
  render(){
    return ( <div>{this.props.children}</div> )
  }
});

const GrvDialogContent = React.createClass({
  render(){
    return ( <div>{this.props.children}</div> )
  }
});

const GrvDialogFooter = React.createClass({
  render(){
    let {onClose, children} = this.props;
    let $content = children;
    if(!children){
      $content = (
        <button onClick={onClose} ref="cancel" type="button" className="btn btn-white">
          Close
        </button>
      )
    }

    return (
      <div className="modal-footer">
        { $content }
      </div>
    )
  }
});

const GrvDialog = React.createClass({

  componentWillUnmount(){
    $(this.refs.modal).modal('hide');
  },

  componentDidMount(){
    $(this.refs.modal).modal('show');
  },

  render() {
    let { title, onClose, className='' } = this.props;
    let $header = <GrvDialogDefaultHeader title={title} />
    let $footer = <GrvDialogFooter onClose={onClose}/>;
    let $content = null;

    className = `modal inmodal grv-dialog ${className}`;

    React.Children.forEach(this.props.children, (child) => {
      if (child == null) {
        return;
      }

      if(child.type.displayName === 'GrvDialogFooter'){
        $footer = child;
      }

      if(child.type.displayName === 'GrvDialogContent'){
        $content = (
          <div className="modal-body">
            <div className="grv-dialog-content">
              {child}
            </div>
          </div>
        )
      }

      if(child.type.displayName === 'GrvDialogHeader'){
        $header = child;
      }
    });

    return (
      <div ref="modal" className={className} data-keyboard="false" data-backdrop="static" tabIndex={-1} role="dialog">
        <div className="modal-dialog">
          <div className="modal-content">
            <div className="modal-header grv-dialog-content-header">
              {$header}
            </div>
            {$content}
            {$footer}
          </div>
        </div>
      </div>
    );
  }
});

module.exports = {
  GrvDialogHeader,
  GrvDialogContent,
  GrvDialogFooter,
  GrvDialog
}
