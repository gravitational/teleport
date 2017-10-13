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

import React from 'react';
import connect from '../connect';
import * as Messages from '../msgPage.jsx';
import getters from '../../flux/settings/getters';

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