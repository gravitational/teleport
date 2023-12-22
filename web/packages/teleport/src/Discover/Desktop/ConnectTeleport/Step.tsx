/**
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

export const StepContainer = styled.div`
  width: 100%;
  display: flex;
  overflow-x: hidden;
  padding-bottom: 50px;
  margin-top: -24px;
  padding-top: 24px;
`;

export const StepTitle = styled.div`
  display: inline-flex;
  align-items: center;
  transition: 0.2s ease-in opacity;
  cursor: pointer;
  font-size: 18px;
  margin-bottom: 30px;
`;

export const StepTitleIcon = styled.div`
  font-size: 30px;
  margin-right: 20px;
`;

export const StepContent = styled.div`
  display: flex;
  flex: 1;
  flex-direction: column;
  margin-right: 30px;
`;

export const StepAnimation = styled.div`
  flex: 0 0 600px;
  margin-left: 30px;
`;

export const StepInstructions = styled.div``;
