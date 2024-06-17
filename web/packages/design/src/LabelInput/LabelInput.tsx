/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import styled from 'styled-components';
import PropTypes from 'prop-types';

import { space, SpaceProps } from 'design/system';

interface LabelInputProps extends SpaceProps {
  hasError?: boolean;
}

const LabelInput = styled.label<LabelInputProps>`
  color: ${props =>
    props.hasError
      ? props.theme.colors.error.main
      : props.theme.colors.text.main};
  display: block;
  font-size: ${p => p.theme.fontSizes[1]}px;
  width: 100%;
  ${space}
`;

LabelInput.propTypes = {
  hasError: PropTypes.bool,
};

LabelInput.defaultProps = {
  hasError: false,
  mb: 1,
};

LabelInput.displayName = 'LabelInput';

export default LabelInput;
