
import React from 'react';
import hoistStatics from 'hoist-non-react-statics'
    
function getDisplayName(WrappedComponent) {
  return WrappedComponent.displayName || WrappedComponent.name || 'Component'
}

export default function withChangeTracker(WrappedComponent) {
  
  class WithChangeTracker extends React.Component {
  
    static displayName = `withChangeTracker(${getDisplayName(WrappedComponent)})`
    
    static contextTypes = {
      changeTracker: React.PropTypes.object.isRequired      
    };
          
    render() {
      const changeTracker = this.context.changeTracker            
      const props = this.props;
      return React.createElement(WrappedComponent, {
        ...props,
        changeTracker
      });       
    }
  }

  return hoistStatics(WithChangeTracker, WrappedComponent)
}