import React from 'react';
import ConfirmChangesDialog from './confirmChangesDialog';

class ChangeTracker extends React.Component {

  static childContextTypes = {
    changeTracker: React.PropTypes.object.isRequired  
  }

  static contextTypes = {
    router: React.PropTypes.object.isRequired      
  };

  register = instance => {
    this._dirtyChildren.push(instance);
  }

  unregister = instance => {
    let index = this._dirtyChildren.indexOf(instance);
    if (index !== -1) {
      this._dirtyChildren.splice(index, 1);
    }
  }

  onCloseConfirmDialog = () => {
    this.setState({ isConfirmDialogVisiable: false });
  }

  onRouterWillLeave = nextLocation => {
    if (nextLocation.state && nextLocation.state.ignore === true) {
      return true;
    }

    return !this.checkIfUnsafedData(() => {
      nextLocation.state = { ignore: true };
      setTimeout(() => this.context.router.push(nextLocation), 0);
    });
  }

  constructor(props) {
    super(props);          
    this._unsubscribe = () => false;
    this._dirtyChildren = [];
    this.state = {
      isConfirmDialogVisiable: false,
      onConfirmDialogOk: () => { }      
    }
  }
  
  getChildContext() {
    return {
      changeTracker: this      
    };
  }

  componentDidMount() {    
    this._unsubscribe = this.context.router.setRouteLeaveHook(
      this.props.route,
      this.onRouterWillLeave);
  }

  componentWillUnmount(){
    this._unsubscribe();
    this._dirtyChildren = [];
  }
          
  checkIfUnsafedData(cb) {
    let hasChanges = this._dirtyChildren.some(inst => inst.hasChanges());

    if (hasChanges) {
      let wrapperCb = () => {
        this.onCloseConfirmDialog();
        cb();
      }

      this.setState({
        isConfirmDialogVisiable: true,
        onConfirmDialogOk: wrapperCb
      })
    } else {
      cb();
    }

    return hasChanges;
  }

  render() {    
    const { isConfirmDialogVisiable, onConfirmDialogOk } = this.state;
    const className = this.props.className || "";
    return (
      <div className={className}>
        {this.props.children}
        <ConfirmChangesDialog
          isVisible={isConfirmDialogVisiable}
          onOk={onConfirmDialogOk}
          onCancel={this.onCloseConfirmDialog} />
      </div>
    );
  }
}
  
export default ChangeTracker;