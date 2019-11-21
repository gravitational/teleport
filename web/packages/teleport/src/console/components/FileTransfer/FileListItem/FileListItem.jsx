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
import * as Icons from 'design/Icon';
import { Box } from 'design';
import { colors } from 'teleport/console/components/colors';
import useHttpTransfer, {
  isProcessing,
  isError,
  isCompleted,
} from 'teleport/console/components/FileTransfer/useHttpTransfer';

export default function FileListItem(props) {
  const { file, onUpdate } = props;
  const { name, id, isUpload, error, url, blob, status } = file;

  const saved = React.useRef(false);

  const httpStatus = useHttpTransfer({
    blob,
    url,
    isUpload,
  });

  React.useEffect(() => {
    const { state, response } = httpStatus;
    if (isCompleted(state) && !isUpload) {
      if (!saved.current) {
        saved.current = true;
        saveOnDisk(response.fileName, response.blob);
      }
    }

    onUpdate({ id, status: httpStatus.state, error: httpStatus.error });
  }, [httpStatus.state]);

  function onRemove() {
    props.onRemove(id);
  }

  const completed = isCompleted(status);
  const failed = isError(status);
  const processing = isProcessing(status);

  let statusText = `${httpStatus.progress}%`;
  if (failed) {
    statusText = 'failed';
  }

  if (completed) {
    statusText = 'completed';
  }

  return (
    <Box mt="4px">
      <Progress>
        <ProgressIndicator
          isCompleted={completed}
          progress={httpStatus.progress}
        >
          {name}
        </ProgressIndicator>
        {processing && <CancelButton onClick={onRemove} />}
        <ProgressStatus isFailed={failed}>{statusText}</ProgressStatus>
      </Progress>
      {failed && <StyledError>{error}</StyledError>}
    </Box>
  );
}

const CancelButton = ({ onClick }) => {
  return (
    <StyledButton onClick={onClick}>
      <Icons.Close />
    </StyledButton>
  );
};

const StyledError = styled.div`
  height: 16px;
  line-height: 16px;
  color: ${colors.error};
`;

const Progress = styled.div`
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
`;

const ProgressStatus = styled.div`
  font-size: 12px;
  height: 24px;
  line-height: 24px;
  width: 80px;
  text-align: right;
  color: ${props => (props.isFailed ? colors.error : colors.terminal)};
`;

const ProgressIndicator = styled.div`
  display: flex;
  align-items: center;
  word-break: break-word;
  background-image: linear-gradient(
    to right,
    ${colors.terminalDark} 0%,
    ${colors.terminalDark} ${props => props.progress}%,
    ${colors.bgTerminal} 0%,
    ${colors.bgTerminal} 100%
  );

  background: ${props => (props.isCompleted ? 'none' : '')};
  color: ${props => (props.isCompleted ? colors.inverse : colors.terminal)};

  min-height: 24px;
  line-height: 1.75;
  width: 360px;
`;

const StyledButton = styled.button`
  background: ${colors.error};
  border-radius: 2px;
  border: none;
  color: ${colors.light};
  cursor: pointer;
  font-size: 12px;
  height: 12px;
  outline: none;
  padding: 0;
  width: 12px;
  &:hover {
    background: ${colors.error};
  }
`;

FileListItem.propTypes = {
  file: PropTypes.object.isRequired,
  onRemove: PropTypes.func.isRequired,
  onUpdate: PropTypes.func.isRequired,
};

function saveOnDisk(fileName, blob) {
  // if IE11
  if (window.navigator.msSaveOrOpenBlob) {
    window.navigator.msSaveOrOpenBlob(blob, fileName);
    return;
  }

  const a = document.createElement('a');
  a.href = window.URL.createObjectURL(blob);
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}
