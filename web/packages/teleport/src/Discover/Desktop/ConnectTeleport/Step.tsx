/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
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
