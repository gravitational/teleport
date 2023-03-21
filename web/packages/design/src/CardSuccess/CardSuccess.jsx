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

import Card from 'design/Card';
import Text from 'design/Text';
import { CircleCheck } from 'design/Icon';

export default function CardSuccess({ title, children }) {
  return (
    <Card width="540px" p={7} my={4} mx="auto" textAlign="center">
      <CircleCheck mb={3} fontSize={56} color="success" />
      {title && (
        <Text typography="h2" mb="4">
          {title}
        </Text>
      )}
      {children}
    </Card>
  );
}

export function CardSuccessLogin() {
  return (
    <CardSuccess title="Login Successful">
      You have successfully signed into your account. <br /> You can close this
      window and continue using the product.
    </CardSuccess>
  );
}
