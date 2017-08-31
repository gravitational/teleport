import React from 'react';
import reactor from 'app/reactor';
import Indicator from 'app/components/indicator';
import { RestRespCodeEnum } from 'app/services/enums';
import * as Messages from 'app/components/msgPage.jsx';

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

      feature.componentDidMount();
    }
    
    componentWillUnmount() {
      this._unsubscribeFn();
    }
             
    render() {      
      if (feature.isProcessing()) {
        return <Indicator delay="long" type="bounce" />;  
      }

      if (feature.isFailed()) {
        const errorText = feature.getErrorText();
        if (feature.getErrorCode() === RestRespCodeEnum.FORBIDDEN) {
          return <Messages.AccessDenied message={errorText}/>  
        }
        return <Messages.Failed message={errorText}/>
      }
      
      const props = this.props;
      return React.createElement(component, {
        ...props,
        feature
      });      
    }
  }
}

export default withFeature;