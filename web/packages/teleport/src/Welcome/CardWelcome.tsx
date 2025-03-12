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

import { ButtonPrimary, H2, Subtitle2 } from 'design';

import { OnboardCard } from 'teleport/components/Onboard';

export function CardWelcome({ title, subTitle, btnText, onClick }: Props) {
  return (
    <OnboardCard center>
      <H2 mb={3}>{title}</H2>
      <Subtitle2 mb="16px">{subTitle}</Subtitle2>
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
