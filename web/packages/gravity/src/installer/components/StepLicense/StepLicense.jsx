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
import styled from 'styled-components';
import PropTypes from 'prop-types';
import { StepLayout } from '../Layout';
import { Text, ButtonPrimary } from 'design';
import { Danger } from 'design/Alert';
import { useAttempt } from 'shared/hooks';
import { useServices } from 'gravity/installer/services';

export function StepLicense({store})  {
  const [ attempt, attemptActions ] = useAttempt();
  const [ license, setLicense ] = React.useState('');
  const { licenseHeaderText } = store.state.config;
  const { isProcessing, isFailed, message } = attempt;
  const btnDisabled = isProcessing || !license;
  const service = useServices();

  function onContinue(){
    attemptActions
      .do(() => {
        return service.setDeploymentType(license, store.state.app.packageId);
      })
      .done(() => {
        store.setLicense(license)
      });
  }

  return (
    <StepLayout title={licenseHeaderText}>
      { isFailed && <Danger> {message }</Danger> }
      <StyledLicense as="textarea" px="2" py="2"  mb="4"
        value={license}
        autoComplete="off"
        onChange={ e => setLicense(e.target.value)}
        typography="body1"
        mono
        bg="light"
        placeholder="Insert your license key here"
        color="text.onLight">
      </StyledLicense>
      <ButtonPrimary width="200px" disabled={btnDisabled} onClick={onContinue}>
        Continue
      </ButtonPrimary>
    </StepLayout>
  );
}

StepLicense.propTypes = {
  label: PropTypes.string.isRequired,
}

const StyledLicense = styled(Text)`
  border-radius: 6px;
  min-height: 200px;
  overflow: auto;
  white-space: pre;
  word-break: break-all;
  word-wrap: break-word;
`;

export default StepLicense;