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

import styled from 'styled-components';

import { MarginProps, SpaceProps, WidthProps } from 'styled-system';
import { ExecutionProps } from 'styled-components/dist/types';

import { space, width } from 'design/system';

const kind = (props: ExecutionProps & AlertProps) => {
  const { kind, theme } = props;
  switch (kind) {
    case 'danger':
      return {
        background: theme.colors.error.main,
        color: theme.colors.buttons.warning.text,
      };
    case 'info':
      return {
        background: theme.colors.info,
        color: theme.colors.text.primaryInverse,
      };
    case 'warning':
      return {
        background: theme.colors.warning.main,
        color: theme.colors.text.primaryInverse,
      };
    case 'success':
      return {
        background: theme.colors.success,
        color: theme.colors.text.primaryInverse,
      };
    default:
      return {
        background: theme.colors.error.main,
        color: theme.colors.text.primaryInverse,
      };
  }
};

interface AlertBaseProps {
  kind?: 'danger' | 'info' | 'warning' | 'success';
}

export type AlertProps = AlertBaseProps & SpaceProps & WidthProps & MarginProps;

const Alert = styled.div<AlertProps>`
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

Alert.defaultProps = {
  kind: 'danger',
};

Alert.displayName = 'Alert';

export default Alert;
export const Danger = styled(Alert).attrs({ kind: 'danger' })<
  Omit<AlertProps, 'kind'>
>``;
export const Info = styled(Alert).attrs({ kind: 'info' })<
  Omit<AlertProps, 'kind'>
>``;
export const Warning = styled(Alert).attrs({ kind: 'warning' })<
  Omit<AlertProps, 'kind'>
>``;
export const Success = styled(Alert).attrs({ kind: 'success' })<
  Omit<AlertProps, 'kind'>
>``;
