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

import { space, color, width } from 'design/system';
import defaultTheme from 'design/theme';

const kind = props => {
  const { kind, theme } = props;
  switch (kind) {
    case 'danger':
      return {
        background: theme.colors.danger,
        color: theme.colors.text.contrast,
      };
    case 'info':
      return {
        background: theme.colors.info,
        color: theme.colors.text.contrast,
      };
    case 'warning':
      return {
        background: theme.colors.warning,
        color: theme.colors.text.contrast,
      };
    case 'success':
      return {
        background: theme.colors.success,
        color: theme.colors.text.contrast,
      };
    default:
      return {
        background: theme.colors.danger,
        color: theme.colors.text.contrast,
      };
  }
};

const Alert = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 2px;
  box-sizing: border-box;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.24);
  margin: 0 0 24px 0;
  min-height: 40px;
  padding: 8px 16px;
  overflow: auto;
  word-break: break-word;
  line-height: 1.5;
  ${space}
  ${kind}
  ${width}

  a {
    color: ${({ theme }) => theme.colors.light};
  }
`;

Alert.propTypes = {
  kind: PropTypes.oneOf(['danger', 'info', 'warning', 'success']),
  ...color.propTypes,
  ...space.propTypes,
  ...width.propTypes,
};

Alert.defaultProps = {
  kind: 'danger',
  theme: defaultTheme,
};

Alert.displayName = 'Alert';

export default Alert;
export const Danger = props => <Alert kind="danger" {...props} />;
export const Info = props => <Alert kind="info" {...props} />;
export const Warning = props => <Alert kind="warning" {...props} />;
export const Success = props => <Alert kind="success" {...props} />;
