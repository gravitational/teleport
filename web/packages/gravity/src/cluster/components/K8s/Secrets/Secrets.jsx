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
import { withState, useAttempt } from 'shared/hooks';
import service from 'gravity/cluster/services/k8s';
import SecretList from './SecretList';
import Indicator from 'design/Indicator';
import { Flex } from 'design';
import { Danger } from 'design/Alert';
import { useK8sContext } from './../k8sContext';
import K8sResourceDialog from './../K8sResourceDialog';

export function Secrets(props) {
  const { attempt, secrets, namespace, onSave } = props;
  const { message, isProcessing, isFailed } = attempt;
  const [ secretToEdit, setSecretToEdit ] = React.useState(null);

  if(isFailed){
    return (
      <Danger>{message} </Danger>
    )
  }

  if(isProcessing){
    return (
      <Flex justifyContent="center">
        <Indicator  />
      </Flex>
    )
  }

  return (
    <React.Fragment>
      <SecretList
        namespace={namespace}
        items={secrets}
        onEdit={setSecretToEdit}
      />
      { secretToEdit && (
        <K8sResourceDialog readOnly={false}
          namespace={secretToEdit.namespace}
          name={secretToEdit.name}
          resource={secretToEdit.resource}
          onClose={ () => setSecretToEdit(null) }
          onSave={onSave}
        />
      )}
    </React.Fragment>
  )
}

export default withState(() => {
  const { namespace } = useK8sContext();
  const [ secrets, setSecrets ] = React.useState([]);
  const [ attempt, attemptActions ] = useAttempt({ isProcessing: true});

  React.useEffect(() => {
    attemptActions.start();
    service.getSecrets(namespace)
      .then(secrets => {
      setSecrets(secrets);
      attemptActions.stop();
    })
    .fail(err => {
      attemptActions.error(err);
    })
  }, [namespace]);

  function onSave(namespace, name, data){
    return service.saveSecret(namespace, name, data)
      .then(() => service.getSecrets(namespace))
      .then(secrets => setSecrets(secrets));
  }

  return {
    secrets,
    attempt,
    onSave
  }
})(Secrets);