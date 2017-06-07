import React from 'react';
import reactor from  'app/reactor';
import ConnectorListItem from './connectorListItem';
import Box from 'app/components/common/boxes/box';
import getters from 'app/flux/settingsAuth/authGetters';
import actions from 'app/flux/settingsAuth/actions';
import ConnectorDeleteDialog from './connectorDeleteDialog';
import EmptyCfg from './../emptyCfg';
import cfg from 'app/config';

const EmptyList = ({ onNew, /*title, description*/ }) => (
  <EmptyCfg
    btnText="Add Provider"
    onClick={onNew}  
    title={(
      <strong>
        There are no OpenID providers configured. Click ‘Add Provider’ button to add one.
      </strong>
    )}
    description={(
      <div>
        OpenID allows participating third party providers (eg, Facebook or Google) to authenticate users, eliminating the need for Teleport specific authentication.        
        <div className="m-t-sm">
          <a target="_blank" href={cfg.oidcDocLink}>
            Learn more in the documentation.
            </a>
        </div>
      </div>
    )}    
  />
);
  
const ConnectorList = React.createClass({

  mixins: [reactor.ReactMixin],

  getDataBindings() {
    return {
      deleteAttemp: getters.deleteConnectorAttemp,
      store: getters.store
    }
  },
  
  componentWillUnmount() {
    actions.clear();
  },
  
  onNewConnector() {
    actions.addNew();
  },
  
  onCancelNewConnector() {
    actions.cancelNew();    
  },
  
  hasNew(connectors) {
    return connectors.some(i => i.isNew);
  },
  
  renderListItem(connector) {    
    return (
      <ConnectorListItem key={connector.key}
        connector={connector}
        onCancelNew={this.onCancelNewConnector} />
    )
  },
  
  render() {
    let { deleteAttemp, store } = this.state;  
    let { connectors, connectorToDelete } = store;    
    let $connectorItems = connectors.length > 0 ?
      connectors.map(this.renderListItem) : <EmptyList onNew={this.onNewConnector} />
        
    return (
      <Box style={{ position: "relative"}}>     
        <ConnectorDeleteDialog
          onContinue={() => actions.deleteConnector(connectorToDelete)}        
          onCancel={actions.closeDeleteConnectorDialog}        
          connectorId={connectorToDelete}
          attemp={deleteAttemp}
        />        
        <div ref="container">
          {$connectorItems}
        </div>  
      </Box>
    );
  }
  
});

export default ConnectorList;
