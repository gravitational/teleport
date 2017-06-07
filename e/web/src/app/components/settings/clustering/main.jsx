import React, { Component } from 'react';
import { connect } from 'nuclear-js-react-addons';
import Box from 'app/components/common/boxes/box';
import clusterGetters from 'app/flux/settingsClusters/getters';
import clusterActions from 'app/flux/settingsClusters/actions';
import EmptyCfg from './../emptyCfg';
import ClusterItem from './clusterItem';
import ChangeTracker from './../changeTracker';
import cfg from 'app/config';

const EmptyBox = () => (
  <Box>     
    <EmptyCfg
      title={(
        <strong>You do not have any Trusted Clusters configured.</strong>
      )}
      description={(
        <div>
          Teleport allows you to link multiple clusters of servers together, each with their own access restrictions.                    
          <div className="m-t-sm">          
            <a target="_blank" href={cfg.clusterDocLink}> Learn more in documentation.</a>            
          </div>
        </div>
      )}/>      
  </Box>  
);

class Clustering extends Component {
  
  render() {            
    let { clusters } = this.props.store;        
    
    let $clusters = clusters.map(cluster => (
        <ClusterItem key={cluster.key} 
          cluster={cluster}              
          onSave={clusterActions.saveCluster}          
        />              
    ))
            
    return (
      <div className="m-t grv-settings-cluster">
        <ChangeTracker
          router={this.props.router}  
          route={this.props.route}
        >            
          {$clusters.length > 0 ?
            (<Box>
              <Box.Header>
                <h3>Trusted Clusters</h3>
              </Box.Header>
              {$clusters}
            </Box>)
            :
            <EmptyBox />
          }
        </ChangeTracker>  
      </div>          
    );
  }
}

function ToProps() {
  return {        
    saveClusterAttemp: clusterGetters.saveClusterAttemp,          
    store: clusterGetters.store
  }
}

export default connect(ToProps)(Clustering);
export { Clustering }
