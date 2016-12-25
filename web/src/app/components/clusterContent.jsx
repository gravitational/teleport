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

var React = require('react');
var reactor = require('app/reactor');
var siteGetters = require('app/modules/sites/getters');
var appGetters = require('app/modules/app/getters');
var DropDown = require('./dropdown.jsx');
var { setSiteId } = require('app/modules/app/actions');

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
  },

  render() {
    let { sites, siteId } = this.state;    
    let siteOptions = sites.map(s => ({ label: s.name, value: s.name }));    
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
