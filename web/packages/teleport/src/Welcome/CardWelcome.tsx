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
import { ButtonPrimary, Text } from 'design';

import { OnboardCard } from 'design/Onboard/OnboardCard';

export function CardWelcome({ title, subTitle, btnText, onClick }: Props) {
  return (
    <OnboardCard center>
      <Text mb="8px" typography="h4">
        {title}
      </Text>
      <Text mb="16px" typography="subtitle1" fontWeight="light">
        {subTitle}
      </Text>
      <ButtonPrimary width="100%" onClick={onClick}>
        {btnText}
      </ButtonPrimary>
    </OnboardCard>
  );
}

type Props = {
  title: string;
  subTitle: string;
  btnText: string;
  onClick(): void;
};
