import React from 'react';
import reactor from 'app/reactor';
import Indicator from 'telebase-app/components/indicator';
import * as Alerts from 'app/components/common/alerts';

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
        const message = feature.getErrorText();
        return <Alerts.Danger className="m-t">{message}</Alerts.Danger>                
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