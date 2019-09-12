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
import { StepLayout } from '../Layout';
import { useInstallerContext } from './../store';
import { Flex } from 'design';
import { useFluxStore } from 'gravity/components/nuclear';
import opProgressGetters from 'gravity/flux/opProgress/getters';
import LogViewer from 'gravity/components/LogViewer';
import AjaxPoller from 'gravity/components/AjaxPoller';
import * as progressActions from 'gravity/flux/opProgress/actions';
import InstallLogsProvider from './InstallLogsProvider';
import ExpandPanel from './TogglePanel';
import ProgressBar from './ProgressBar';
import ProgressDesc from './ProgressDescription';
import Completed from './Completed';
import Failed from './Failed';

export function StepProgress(props) {
  const { progress, logProvider, ...styles } = props;
  const [ showLogs, toggleLogs ] = React.useState(false);

  function onToggleLogs(){
    toggleLogs(!showLogs)
  }

  const {
    isError,
    isCompleted,
    step,
    siteId,
    crashReportUrl
  } = progress;

  const progressValue = (100 / (PROGRESS_STATE_STRINGS.length)) * (step + 1);
  const isInstalling = !(isError || isCompleted);
  const title = isInstalling ? "Installation" : '';

  return (
    <StepLayout title={title} height="100%" {...styles}>
      { isCompleted && <Completed siteId={siteId}/> }
      { isError && <Failed tarballUrl={crashReportUrl}/> }
      { isInstalling && (
        <>
          <ProgressBar mb="4" value={progressValue} />
          <ProgressDesc step={step} steps={PROGRESS_STATE_STRINGS} />
        </>
      )}
      <ExpandPanel
        mt="4"
        title="Executable Logs"
        expanded={showLogs}
        onToggle={onToggleLogs}
        height={ showLogs ? "100%" : "auto"  }
      >
        <Flex pt="2" px="2" minHeight="400px" height="100%" bg="bgTerminal" style={{ display: showLogs ? 'inherit': 'none' }} >
          <LogViewer
            autoScroll={true}
            provider={ logProvider }
          />
        </Flex>
      </ExpandPanel>
    </StepLayout>
  );
}

// state provider
export default function(props) {
  const { state } = useInstallerContext();
  const { siteId, id } = state.operation;
  const progress = useFluxStore(opProgressGetters.progressById(id));

  // poll operation progress status
  function onFetchProgress(){
    return progressActions.fetchOpProgress(siteId, id)
  }

  // creates web socket connection and streams install logs
  const $provider = (
    <InstallLogsProvider siteId={siteId} opId={id} />
  )

  return (
    <>
      { progress && (
        <StepProgress
          {...props}
          progress={progress}
          logProvider={$provider}
        />
      )}
      <AjaxPoller time={POLL_INTERVAL} onFetch={onFetchProgress} />
    </>
  )
}

const POLL_INTERVAL = 3000; // every 5 sec

export const PROGRESS_STATE_STRINGS = [
  'Provisioning Instances',
  'Connecting to instances',
  'Verifying instances',
  'Preparing configuration',
  'Installing dependencies',
  'Installing platform',
  'Installing application',
  'Verifying application',
  'Connecting to application'
];