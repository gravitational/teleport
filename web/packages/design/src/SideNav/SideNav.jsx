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

const SideNav = styled.nav`
  background: ${props => props.theme.colors.levels.surface};
  border-right: 1px solid ${props => props.theme.colors.levels.sunkenSecondary};
  min-width: 240px;
  width: 240px;
  overflow: auto;
  height: 100%;
  display: flex;
  flex-direction: column;
`;

SideNav.displayName = 'SideNav';

export default SideNav;
