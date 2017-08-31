import React from 'react';
import reactor from 'app/reactor';
import Indicator from 'app/components/indicator';
import { Failed } from 'app/components/msgPage.jsx';

const withFeature = feature => component => {
  
  return class WithFeatureWrapper extends React.Component{
      
    static displayName = `withFeatureWrapper`

    constructor(props, context) {
      super(props, context)            
      this._unsubscribeFn = null;
    }
                    
    componentDidMount() {
      this._unsubscribeFn = reactor.observe(feature.initAttemptGetter(), ()=>{        
        this.setState({})
      })
    }

    componentWillUnmount() {
      this._unsubscribeFn();
    }
             
    render() {      
      if (feature.isProcessing()) {
        return <Indicator delay="long" type="bounce" />;  
      }

      if (feature.isFailed()) {
        return <Failed message={feature.getErrorText()}/>
      }
      
      let props = this.props;
      return React.createElement(component, {
        ...props,
        feature
      });      
    }
  }
}

export default withFeature;