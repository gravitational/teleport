/*
 *
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

import { Card, CardSuccess, Text } from 'design';
import { CircleStop } from 'design/Icon';
import React from 'react';

export function CardDenied({ title, children }) {
  return (
    <Card width="540px" p={7} my={4} mx="auto" textAlign="center">
      <CircleStop mb={3} fontSize={56} color="red" />
      {title && (
        <Text typography="h2" mb="4">
          {title}
        </Text>
      )}
      {children}
    </Card>
  );
}

export function CardAccept({ title, children }) {
  return <CardSuccess title={title}>{children}</CardSuccess>;
}
