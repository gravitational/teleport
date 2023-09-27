/*
Copyright 2022 Gravitational, Inc.

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

// NOTE: This MainContainer component is imported in multiple places and then
// modified using styled-components. If it's exported from the Main/index.ts
// or included in the Main.tsx file there is a circular dependency that causes
// odd issues around the MainContainer component being available at certain
// times.
export const MainContainer = styled.div`
  display: flex;
  flex: 1;
  min-height: 0;
  --sidebar-width: 256px;
`;
