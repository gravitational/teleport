import React from 'react';
import connect from 'telebase-app/components/connect';
import * as Messages from 'telebase-app/components/msgPage.jsx';
import getters from 'app/flux/settings/getters';

class SettingsIndex extends React.Component {      
  
  static propTypes = {
    router: React.PropTypes.object.isRequired,
    store: React.PropTypes.object.isRequired,
    location: React.PropTypes.object.isRequired,
  }
  
  componentDidMount(){    
    const route = this.getAvailableRoute();
    if(route){      
      this.props.router.replace({ pathname: route })
    }
  }

  getAvailableRoute(){    
    const items = this.props.store.getNavItems();    
    if(items && items[0]){      
      return items[0].to;
    }
    
    return null;
  }

  render(){        
    return ( <Messages.AccessDenied/> )
  }
}

function mapStateToProps() {
  return {    
    store: getters.store    
  }
}

export default connect(mapStateToProps)(SettingsIndex);