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

import { fontSize, color, width, space } from 'design/system';
import { fade } from 'design/theme/utils/colorManipulator';

const kinds = ({ theme, kind, shadow }) => {
  // default is primary
  const styles = {
    background: theme.colors.brand,
    color: theme.colors.text.contrast,
  };

  if (kind === 'secondary') {
    styles.background = theme.colors.levels.sunkenSecondary;
    styles.color = theme.colors.text.main;
  }

  if (kind === 'warning') {
    styles.background = theme.colors.warning.main;
    styles.color = theme.colors.text.contrast;
  }

  if (kind === 'danger') {
    styles.background = theme.colors.danger;
    styles.color = theme.colors.text.contrast;
  }

  if (kind === 'success') {
    styles.background = theme.colors.success;
    styles.color = theme.colors.text.contrast;
  }

  if (shadow) {
    styles.boxShadow = `
    0 0 8px ${fade(styles.background, 0.24)},
    0 4px 16px ${fade(styles.background, 0.56)}
    `;
  }

  return styles;
};

const LabelState = styled.span`
  box-sizing: border-box;
  border-radius: 100px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 16px;
  line-height: 1.4;
  padding: 0 8px;
  font-size: 10px;
  font-weight: 500;
  text-transform: uppercase;
  ${space}
  ${kinds}
  ${width}
  ${color}
  ${fontSize}
`;
LabelState.defaultProps = {
  fontSize: 0,
  color: 'light',
  fontWeight: 'bold',
  shadow: false,
};

export default LabelState;
export const StateDanger = props => <LabelState kind="danger" {...props} />;
export const StateInfo = props => <LabelState kind="secondary" {...props} />;
export const StateWarning = props => <LabelState kind="warning" {...props} />;
export const StateSuccess = props => <LabelState kind="success" {...props} />;
