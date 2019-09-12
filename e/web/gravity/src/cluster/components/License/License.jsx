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
import { forEach } from 'lodash';
import PropTypes from 'prop-types';
import htmlUtils from 'gravity/lib/htmlUtils';
import { useFluxStore } from 'gravity/components/nuclear';
import { getters } from 'gravity/flux/cluster';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from 'gravity/cluster/components/Layout';
import { updateLicense } from 'e-gravity/cluster/flux/actions';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { Text, Flex, Box } from 'design';
import { withState } from 'shared/hooks';
import * as Icons from 'design/Icon';
import UpdateLicenseDialog from './UpdateLicenseDialog';

export function ClusterLicense(props){
  const { onUpdateLicense, license } = props;
  const { info = {}, status } = license
  const $infoItems = [];
  const [ isOpenDialog, setIsOpen ] = React.useState(false);

  forEach(info, (value, title) => {
    $infoItems.push(<LicenseAttr key={title} title={title} value={value}/>)
  });

  function onDownloadLicense(){
    const { raw } = license;
    htmlUtils.download('license.txt', raw );
  }

  function onUpdate(newLicense){
    return onUpdateLicense(newLicense);
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          LICENSE
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Flex bg="primary.light" p="3" alignItems="center">
        <Icons.License color="text.primary" fontSize={8} mr={2}/>
        <Text typography="h4">
          CLUSTER LICENSE
        </Text>
        <Box ml="auto">
          <ButtonSecondary mr="3" onClick={onDownloadLicense}>
            DOWNLOAD LICENSE
          </ButtonSecondary>
          <ButtonPrimary onClick={ () => setIsOpen(true)}>
            UPDATE LICENSE
          </ButtonPrimary>
        </Box>
      </Flex>
      <Box bg="primary.main" p="3">
        <LicenseStatus status={status} onUpdate={onUpdate}/>
        {$infoItems}
      </Box>
      { isOpenDialog && <UpdateLicenseDialog onClose={() => setIsOpen(false)}/> }
    </FeatureBox>
  )
}

ClusterLicense.propTypes = {
  license: PropTypes.object.isRequired,
}

function LicenseStatus(props){
  const { isError } = props.status;
  const title = 'STATUS';
  if(isError){
    return <LicenseAttr title={title} warning value="INVALID LICENSE"/>
  }

  return <LicenseAttr title={title} value="ACTIVE"/>
}

function LicenseAttr(props){
  const { title, value, warning } = props;
  const color = warning ? 'danger' : 'primary.contrastText';
  return (
    <Flex alignItems="center" mb="3">
      <Text typography="h6" color={color} mr="2" caps>
        {title}:
      </Text>
      <Text color={color}>
        {value}
      </Text>
    </Flex>
  )
}

const mapState = () => {
  const store = useFluxStore(getters.clusterStore)
  return {
    onUpdateLicense: updateLicense,
    license: store.cluster.license,
  }
}

export default withState(mapState)(ClusterLicense);