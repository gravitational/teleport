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
import * as Icons from 'design/Icon';
import { ButtonWarning, Card, Box, Text, Flex } from 'design';
import { withState } from 'shared/hooks';
import { useFluxStore } from 'gravity/components/nuclear';
import { getters } from 'gravity/cluster/flux/tlscert';
import { FeatureBox, FeatureHeader, FeatureHeaderTitle } from './../Layout';
import UpdateCertDialog from './UpdateCertDialog';

export function Certificate(props) {
  const { store, } = props;
  const [ isOpen, setIsOpen ] = React.useState(false);
  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          HTTPS Certificate
        </FeatureHeaderTitle>
      </FeatureHeader>
      <Card bg='primary.light'>
        <Flex borderTopRightRadius="3" borderTopLeftRadius="3" pl={6} pr={4} py={4} alignItems="center">
          <Icons.License color="text.primary" fontSize={8} mr={2}/>
          <Text typography="subtitle1" bold>
            {store.getToCn()}
          </Text>
          <ButtonWarning size="small" ml="auto" onClick={() => setIsOpen(true)}>
            Replace
          </ButtonWarning>
        </Flex>
        <Box bg='primary.main' p={6} mb={-2} borderBottomRightRadius="3" borderBottomLeftRadius="3">
          <Text typography="body1" bold mb={2}>
            Issued To
          </Text>
          <CertAttr name="Common Name (CN)" value={store.getToCn()} />
          <CertAttr name="Organization (O)" value={store.getToOrg()} />
          <CertAttr name="Organization Unit (OU)" value={store.getToOrgUnit()} />
          <Text typography="body1" bold mt={4} mb={2}>
            Issued by
          </Text>
          <CertAttr name="Organization (O)" value={store.getByOrg()} />
          <CertAttr name="Organization Unit (OU)" value={store.getByOrgUnit()} />
          <Text typography="body1" bold mt={4} mb={2}>
            Validity Period
          </Text>
          <CertAttr name="Issued On" value={store.getStartDate()} />
          <CertAttr name="Expires On" value={store.getEndDate()} />
        </Box>
      </Card>
      { isOpen && <UpdateCertDialog onClose={ () => setIsOpen(false) } /> }
    </FeatureBox>
  )
}

const CertAttr = ({ name, value }) => (
  <Box mb={2}>
    <span style={{ width: "180px", display: "inline-block" }}>{name}</span>
    <span>{value}</span>
  </Box>
)

export default withState(() => {
  const store = useFluxStore(getters.store);
  return {
    store
  }
})(Certificate);