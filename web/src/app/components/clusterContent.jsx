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
import reactor from 'app/reactor';
import siteGetters from 'app/modules/sites/getters';
import appGetters from 'app/modules/app/getters';
import DropDown from './dropdown.jsx';
import { setSiteId, refresh } from 'app/modules/app/actions';

const YOUR_CSR_TEXT = 'Your cluster';

var PageWithHeader = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {                  
      sites: siteGetters.sites,
      siteId: appGetters.siteId
    }
  },

  onChangeSite(value) {
    setSiteId(value);      
    refresh();
  },

  render() {
    let { sites, siteId } = this.state;    
    let siteOptions = sites.map((s, index) => ({
      label: index === 0 ? YOUR_CSR_TEXT : s.name,
      value: s.name
    }));
        
    return (                  
      <div className="grv-page">        
        <DropDown
          className="grv-page-header-clusters-selector m-t-sm"
          size="sm"      
          align="right"
          onChange={this.onChangeSite}
          value={siteId}
          options={siteOptions}
          />
        {this.props.children}
      </div>
    );
  }
});

export default PageWithHeader;
