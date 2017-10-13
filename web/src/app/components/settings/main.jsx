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

import React, { PropTypes } from 'react';
import { connect } from 'nuclear-js-react-addons';
import { Link } from 'react-router';
import getters from '../../flux/settings/getters';

const Separator = () => <div className="grv-settings-header-line-solid m-t-sm m-b-sm"/>;

class Settings extends React.Component {

  static contextTypes = {
    router: PropTypes.object.isRequired
  }
        
  renderHeaderItem(item, key){
    let { to, isIndex, title } = item;    
    let className = this.context.router.isActive(to, isIndex) ? "active" : "";
    return (
      <li key={key} className={className}>
        <Link to={to}>
          <h2 className="m-b-xxs">
            {title}
          </h2>
        </Link>
        <Separator/>
      </li>
    )
  }
  
  render() {        
    const { store } = this.props;                 
    const $headerItems = store.getNavItems().map(this.renderHeaderItem.bind(this));

    if ( !store.isReady() ){
      return null;
    }
    
    return (   
      <div className="grv-page grv-settings">                                                
        <ul className="grv-settings-header-menu">
          {$headerItems}
        </ul>  
        { $headerItems.length > 0 && <Separator /> }
        {this.props.children}        
      </div>
    );
  }
}

function mapStateToProps() {
  return {    
    store: getters.store
  }
}

export default connect(mapStateToProps)(Settings);
