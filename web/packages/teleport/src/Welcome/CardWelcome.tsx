/*
Copyright 2021 Gravitational, Inc.

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
import { Card, Box, Text, ButtonPrimary } from 'design';

export function CardWelcome({ title, subTitle, btnText, onClick }: Props) {
  return (
    <Card bg="levels.surface" my={6} mx="auto" width="464px">
      <Box p={6}>
        <Text typography="h2" mb={3} textAlign="center" color="light">
          {title}
        </Text>
        <Text typography="h5" mb={3} textAlign="center">
          {subTitle}
        </Text>
        <ButtonPrimary width="100%" mt={3} size="large" onClick={onClick}>
          {btnText}
        </ButtonPrimary>
      </Box>
    </Card>
  );
}

type Props = {
  title: string;
  subTitle: string;
  btnText: string;
  onClick(): void;
};
