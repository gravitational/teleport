import React from 'react';
import { Store, useStore } from 'gravity/lib/stores';

export default class HubClusterStore extends Store {

  state = {
    clusterToDisconnect: null,
  }

  setClusterToDisconnect = clusterToDisconnect => {
    this.setState({
      clusterToDisconnect
    })
  }
}

const hubClusterContext =  React.createContext({});

export const Provider = hubClusterContext.Provider;

export function useHubClusterContext(){
  return React.useContext(hubClusterContext);
}

export {
  useStore
}

