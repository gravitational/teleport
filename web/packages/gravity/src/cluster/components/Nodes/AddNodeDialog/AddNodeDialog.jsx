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

import React from 'react'
import PropTypes from 'prop-types';
import { Box } from 'design';
import { withState } from 'shared/hooks';
import { getters } from 'gravity/flux/cluster';
import { getters as infoGetters } from 'gravity/cluster/flux/info';
import { useFluxStore } from 'gravity/components/nuclear';
import Dialog, { DialogTitle, DialogHeader } from 'design/DialogConfirmation';
import { ExpandPolicyEnum } from 'gravity/services/enums';
import ProfileSelector from './ProfileSelector';
import ProfileInstructions from './ProfileInstructions';

export class AddNodeDialog extends React.Component{

  constructor(props){
    super();

    const profileOptions = props.profiles
      .filter( item => item.expandPolicy !== ExpandPolicyEnum.FIXED)
      .map(item => ({
        value: item.name,
        title: `${item.description || item.serviceRole} (required -${item.requirementsText})`
      }));

    this.state = {
      selectedProfile: profileOptions[0],
      showCommands: null,
      profileOptions
    }
  }

  onContinue = () => {
    this.setState({ showCommands: true})
  }

  setSelectedProfile = selectedProfile => {
    this.setState({selectedProfile})
  }

  render(){
    const { onClose, advertiseIp, joinToken, gravityUrl } = this.props;
    const { selectedProfile, profileOptions, showCommands } = this.state;
    const role = selectedProfile.value;
    const downloadCmd = `curl -k -H "Authorization: Bearer ${joinToken}" ${gravityUrl} -o gravity`;
    const joinCmd = `gravity join ${advertiseIp} --token=${joinToken} --role=${role}`;

    return (
      <Dialog
        disableEscapeKeyDown={false}
        onClose={onClose}
        open={true}
      >
        <Box width="700px">
          <DialogHeader>
            <DialogTitle>
              ADD A NODE
            </DialogTitle>
          </DialogHeader>
          {!showCommands && (
            <ProfileSelector
              onContinue={this.onContinue}
              value={selectedProfile}
              onChange={this.setSelectedProfile}
              onClose={onClose}
              options={profileOptions}
            />
          )}
          {showCommands && (
            <ProfileInstructions downloadCmd={downloadCmd} joinCmd={joinCmd} onClose={onClose}/>
          )}
        </Box>
      </Dialog>
    );
  }
}

AddNodeDialog.propTypes = {
  joinToken: PropTypes.string.isRequired,
  onClose: PropTypes.func.isRequired,
  advertiseIp: PropTypes.string.isRequired,
  gravityUrl: PropTypes.string.isRequired,
}

function mapState() {
  const clusterStore = useFluxStore(getters.clusterStore);
  const infoStore = useFluxStore(infoGetters.infoStore);
  const { advertiseIp, gravityUrl } = infoStore.info;

  return {
    profiles: clusterStore.cluster.nodeProfiles,
    advertiseIp,
    gravityUrl,
  }
}

export default withState(mapState)(AddNodeDialog);