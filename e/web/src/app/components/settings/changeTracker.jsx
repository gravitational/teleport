import React from 'react';
import ConfirmChangesDialog from './../confirmChangesDialog';

class ChangeTracker extends React.Component {

  constructor(props) {
    super(props);

    this.register = this.register.bind(this);
    this.unregister = this.unregister.bind(this);
    this.onCloseConfirmDialog = this.onCloseConfirmDialog.bind(this);

    this.dirtyChildren = [];
    this.state = {
      isConfirmDialogVisiable: false,
      onConfirmDialogOk: () => { }
    }
  }
  
  getChildContext() {
    return {
      register: this.register,
      unregister: this.unregister
    };
  }

  componentDidMount() {
    this.props.router.setRouteLeaveHook(
      this.props.route,
      this.onRouterWillLeave.bind(this));
  }

  onRouterWillLeave(nextLocation) {
    if (nextLocation.state && nextLocation.state.ignore === true) {
      return true;
    }

    return !this.checkIfUnsafedData(() => {
      nextLocation.state = { ignore: true };
      setTimeout(() => this.props.router.push(nextLocation), 0);
    });
  }

  onCloseConfirmDialog() {
    this.setState({ isConfirmDialogVisiable: false });
  }
  
  register(instance) {
    this.dirtyChildren.push(instance);
  }

  unregister(instance) {
    let index = this.dirtyChildren.indexOf(instance);
    if (index !== -1) {
      this.dirtyChildren.splice(index, 1);
    }
  }

  checkIfUnsafedData(cb) {
    let hasChanges = this.dirtyChildren.some(inst => inst.hasChanges());

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
    let { isConfirmDialogVisiable, onConfirmDialogOk } = this.state;
    return (
      <div>
        {this.props.children}
        <ConfirmChangesDialog
          isVisible={isConfirmDialogVisiable}
          onOk={onConfirmDialogOk}
          onCancel={this.onCloseConfirmDialog} />
      </div>
    );
  }
}
  
ChangeTracker.childContextTypes = {
  register: React.PropTypes.func.isRequired,
  unregister: React.PropTypes.func.isRequired
};

export default ChangeTracker;