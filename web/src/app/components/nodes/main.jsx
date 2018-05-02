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
import { connect } from 'nuclear-js-react-addons';
import userAclGetters from 'app/flux/userAcl/getters';
import nodeGetters from 'app/flux/nodes/getters';
import appGetters from 'app/flux/app/getters';
import NodeList from './nodeList.jsx';
import { getters as sshHistoryGetters } from 'app/flux/sshHistory/store';

import withStorage from './../withStorage.jsx';

const Nodes = props => {  
  const { siteNodes, sshHistory, aclStore, sites, siteId, storage } = props;
  const logins = aclStore.getSshLogins().toJS();  
  const nodeRecords = siteNodes.toJS();
  return (   
    <div className="grv-page">
      <NodeList
        sshHistory={sshHistory}
        storage={storage}
        siteId={siteId}
        sites={sites} 
        nodeRecords={nodeRecords} 
        logins={logins}
      />
    </div>
  );
}  

function mapStateToProps() {
  return {    
    siteId: appGetters.siteId,
    siteNodes: nodeGetters.siteNodes,
    aclStore: userAclGetters.userAcl,
    sshHistory: sshHistoryGetters.store
  }
}

const NodesWithStorage = withStorage(Nodes);

export default connect(mapStateToProps)(NodesWithStorage);
