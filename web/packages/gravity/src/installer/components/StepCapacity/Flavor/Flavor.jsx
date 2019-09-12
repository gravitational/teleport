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
import Profile from './Profile';
import PropTypes from 'prop-types';
import AjaxPoller from 'gravity/components/AjaxPoller';
import * as Alerts from 'design/Alert';
import { Flex, ButtonPrimary } from 'design';
import Validation, {
  useValidation,
} from 'shared/components/Validation';
import { useServices } from 'gravity/installer/services';
import { useAttempt } from 'shared/hooks';

const POLL_INTERVAL = 3000; // every 5 sec

export default function Flavor(props) {
  const { servers, profiles, store } = props;

  const [attempt, attemptActions] = useAttempt();
  const service = useServices();

  function onSetServerVars({ role, hostname, ip, mounts }) {
    store.setServerVars({ role, hostname, ip, mounts });
  }

  function onRemoveServerVars({ role, hostname }) {
    store.removeServerVars({ role, hostname });
  }

  const $reqItems = profiles.map(profile => {
    const {
      instructions,
      requirementsText,
      count,
      name,
      description,
    } = profile;

    const profileServers = servers.filter(s => s.role === name);

    return (
      <Profile
        mb="4"
        servers={profileServers}
        key={name}
        requirementsText={requirementsText}
        instructions={instructions}
        count={count}
        name={name}
        description={description}
        onSetServerVars={onSetServerVars}
        onRemoveServerVars={onRemoveServerVars}
      />
    );
  });

  function onContinue() {
    const request = store.makeStartInstallRequest();
    attemptActions.start();
    service
      .startInstall(request)
      .done(() => {
        store.setStepProgress();
      })
      .fail(err => {
        attemptActions.error(err);
      });
  }

  function onFetchAgentReport() {
    const request = store.makeAgentRequest();
    return service
      .fetchAgentReport(request)
      .then(agentServers => {
        store.setAgentServers(agentServers);
      })
      .fail(() => {
        store.setAgentServers([]);
      });
  }

  function onVerfiy() {
    const request = store.makeStartInstallRequest();
    attemptActions.start();
    service
      .verifyOnPrem(request)
      .done(() => {
        attemptActions.stop('Verified!');
      })
      .fail(err => {
        attemptActions.error(err);
      });
  }

  const btnDisabled = attempt.isProcessing;

  return (
    <Validation>
      <>
        {attempt.isFailed && <Alerts.Danger>{attempt.message}</Alerts.Danger>}
        {attempt.isSuccess && (
          <Alerts.Success>{attempt.message}</Alerts.Success>
        )}
        {$reqItems}
        <Flex mt="60px">
          <ButtonValidate
            width="200px"
            mr="3"
            disabled={btnDisabled}
            onClick={onContinue}
          >
            Continue
          </ButtonValidate>
          <ButtonValidate
            disabled={btnDisabled}
            onClick={onVerfiy}
            kind="secondary"
          >
            Verify
          </ButtonValidate>
        </Flex>
      </>
      <AjaxPoller time={POLL_INTERVAL} onFetch={onFetchAgentReport} />
    </Validation>
  );
}

Flavor.propTypes = {
  store: PropTypes.object.isRequired,
  profiles: PropTypes.array.isRequired,
};

function ButtonValidate({ onClick, ...rest }) {
  const validator = useValidation();
  function onContinue() {
    validator.validate() && onClick();
  }

  return <ButtonPrimary onClick={onContinue} {...rest} />;
}
