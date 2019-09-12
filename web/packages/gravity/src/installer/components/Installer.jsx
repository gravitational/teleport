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
import { Flex, Indicator } from 'design';
import Validation from 'shared/components/Validation';
import { Failed } from 'design/CardError';
import {
  Provider as ServiceProvider,
  useServices,
} from 'gravity/installer/services';
import { AppLayout } from './Layout';
import Eula from './Eula';
import InstallerStore, {
  useInstallerStore,
  StepEnum,
  Provider as StoreProvider,
} from './store';
import StepProgress from './StepProgress';
import StepCapacity from './StepCapacity';
import StepList from './StepList';
import StepProvider from './StepProvider';
import StepLicense from './StepLicense';
import Description from './Description';
import Logo from './Logo';

export function Installer(props) {
  const store = useInstallerStore();
  const service = useServices();
  const { repository, name, version, siteId } = props;
  const {
    step,
    stepOptions,
    config,
    app,
    eulaAccepted,
    status,
    statusText,
  } = store.state;

  React.useEffect(() => {
    if (!siteId) {
      service
        .fetchApp(name, repository, version)
        .then(app => store.initWithApp(app))
        .fail(err => store.setError(err));
    } else {
      service
        .fetchClusterDetails(siteId)
        .then(response => {
          store.initWithCluster(response);
        })
        .fail(err => store.setError(err));
    }
  }, []);

  if (status === 'error') {
    return <Failed message={statusText} />;
  }

  if (status !== 'ready') {
    return (
      <AppLayout alignItems="center" justifyContent="center">
        <Indicator />
      </AppLayout>
    );
  }

  if (app.eula && !eulaAccepted) {
    return <Eula onAccept={store.acceptEula} config={config} app={app} />;
  }

  const logoSrc = app.logo;

  return (
    <Validation>
      <AppLayout>
        <Flex
          flex="1"
          px="8"
          py="10"
          mr="4"
          mb="5"
          justifyContent="flex-end"
          style={{ overflow: 'auto' }}
        >
          <Flex flexDirection="column" flex="1" maxWidth="1000px">
            <Flex mb="10" alignItems="center" flexWrap="wrap">
              <Logo src={logoSrc} />
              <StepList value={step} options={stepOptions} />
            </Flex>
            {step === StepEnum.NEW_APP && <StepProvider />}
            {step === StepEnum.LICENSE && <StepLicense store={store} />}
            {step === StepEnum.PROVISION && <StepCapacity />}
            {step === StepEnum.PROGRESS && <StepProgress />}
          </Flex>
        </Flex>
        <Flex flex="0 0 30%" bg="primary.main">
          <Description store={store} />
        </Flex>
      </AppLayout>
    </Validation>
  );
}

export default function Container({ match, service, store }) {
  const { siteId, repository, name, version } = match.params;
  const [installerStore] = React.useState(() => store || new InstallerStore());
  const props = {
    siteId,
    repository,
    name,
    version,
  };

  return (
    <ServiceProvider value={service}>
      <StoreProvider value={installerStore}>
        <Installer {...props} />
      </StoreProvider>
    </ServiceProvider>
  );
}
