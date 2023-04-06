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

import Box from './../Box';
import theme from './../theme';

const Card = styled(Box)`
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.24);
  border-radius: 8px;
`;

Card.defaultProps = {
  theme: theme,
  bg: 'levels.surface',
};

Card.displayName = 'Card';

export default Card;
