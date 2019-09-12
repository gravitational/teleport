/*
Copyright 2019 Gravitational, Inc.

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
import { useFluxStore } from 'gravity/components/nuclear';
import { withState, useAttempt } from 'shared/hooks';
import { Danger } from 'design/Alert'
import { getters } from 'gravity/cluster/flux/nodes';
import { fetchNodes } from 'gravity/cluster/flux/nodes/actions';
import { ButtonPrimary } from 'design';
import NodeList from './NodeList';
import service from 'gravity/cluster/services/info';
import AddNodeDialog from './AddNodeDialog';
import DeleteNodeDialog from './DeleteNodeDialog';
import AjaxPoller from 'gravity/components/AjaxPoller';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';

const POLLING_INTERVAL = 10000; // every 10 sec

export function Nodes({ nodes, onFetch, onFetchToken }){
  // state
  const [ joinToken, setJoinToken ] = React.useState(null);
  const [ isAddNodeDialogOpen, setIsAddNodeDialogOpen ] = React.useState(false);
  const [ nodeToDelete, setNodeToDelete ] = React.useState(null);
  const [ attempt, attemptActions ] = useAttempt();

  // actions
  const openAddDialog = () => {
    attemptActions.do(() => {
      return onFetchToken().then(joinToken => {
        setJoinToken(joinToken);
        setIsAddNodeDialogOpen(true);
      })
    })
  }

  const closeAddDialog = () => setIsAddNodeDialogOpen(false);
  const openDeleteDialog = nodeToDelete => setNodeToDelete(nodeToDelete);
  const closeDeleteDialog = () => setNodeToDelete(null);
  const { isFailed, message, isProcessing } = attempt;

  return (
    <FeatureBox>
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>
          Nodes
        </FeatureHeaderTitle>
        <ButtonPrimary disabled={isProcessing || isAddNodeDialogOpen} ml="auto" width="200px" onClick={openAddDialog}>
          Add Node
        </ButtonPrimary>
      </FeatureHeader>
      { isFailed && <Danger> {message} </Danger> }
      <NodeList onDelete={openDeleteDialog} nodes={nodes} />
      { isAddNodeDialogOpen && ( <AddNodeDialog joinToken={joinToken} onClose={closeAddDialog} /> )}
      { nodeToDelete && <DeleteNodeDialog node={nodeToDelete} onClose={closeDeleteDialog}/>}
      <AjaxPoller time={POLLING_INTERVAL} onFetch={onFetch} />
    </FeatureBox>
  )
}

const mapState = () => {
  const nodeStore = useFluxStore(getters.nodeStore);
  return {
    onFetch: fetchNodes,
    onFetchToken: service.fetchJoinToken,
    nodes: nodeStore.nodes
  }
}

export default withState(mapState)(Nodes);