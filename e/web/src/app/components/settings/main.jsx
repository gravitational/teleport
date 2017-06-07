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
import { isObject } from 'lodash';
// telebase
import { AccessDenied, Failed } from 'telebase-app/components/msgPage.jsx';
import Indicator from 'telebase-app/components/indicator.jsx';
// local
import cfg from 'app/config';
import getters from 'app/flux/settings/getters';
import { RestRespCodeEnum } from 'app/services/enums';

const headerItems = [
  {
    isIndex: true,
    to: cfg.routes.settingsBase,
    title: "Auth"
  },
  {
    to: cfg.routes.settingsRoles,
    title: "Roles"
  },
  {
    to: cfg.routes.settingsCluster,
    title: "Clustering"
  }      
];

const Separator = () => (<div className="grv-settings-header-line-solid m-t-sm m-b-sm"></div>);

class Settings extends React.Component {
        
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
    const { store, initAttemp } = this.props;
    const { isProcessing, isFailed, message } = initAttemp;
    
    if (!store.isReady() && isProcessing) {      
      return (
        <div>
          <Indicator type={'bounce'} />
        </div>
      )
    }

    if (isFailed) {
      if (isObject(message) && message.code === RestRespCodeEnum.FORBIDDEN) {
        return <AccessDenied message={message.text}/>
      } else {
        return <Failed message={message}/>
      }      
    }

    
    if (!store.isReady()) {
      return null;
    }

    let $headerItems = headerItems.map(this.renderHeaderItem.bind(this));
    return (   
      <div className="grv-page grv-settings">
        <ul className="grv-settings-header-menu m-t">
          {$headerItems}
        </ul>  
        <Separator />
        <div>
          {this.props.children}
        </div>
      </div>
    );
  }
}

Settings.contextTypes = {
  router: PropTypes.object.isRequired
}

function mapStateToProps() {
  return {    
    store: getters.store,
    initAttemp: getters.initAttemp      
  }
}

export default connect(mapStateToProps)(Settings);
