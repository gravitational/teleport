import React from 'react';
import ProviderList from './connectorList';
import ChangeTracker from './../changeTracker';

class Auth extends React.Component {
  render() {    
    return (
      <div className="grv-settings-auth">           
        <ChangeTracker
          router={this.props.router}  
          route={this.props.route}>
          <ProviderList />
        </ChangeTracker>        
      </div>
    );
  }    
}

export default Auth;