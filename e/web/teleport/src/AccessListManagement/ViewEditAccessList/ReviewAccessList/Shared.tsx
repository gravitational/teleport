import styled from 'styled-components';

import { Flex, Text } from 'design';

import {
  ReviewDayOfMonthOption,
  ReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';

import { ButtonPencil } from '../Shared';

export enum ReviewStep {
  EditMembershipRequires,
  EditMembers,
  Summary,
}

export type EditedRecurrence = {
  reviewFrequency: ReviewFrequencyOption;
  reviewDayOfMonth: ReviewDayOfMonthOption;
};

export const EditButton = ({
  setStep,
  title,
  disabled,
}: {
  setStep(): void;
  title: string;
  disabled: boolean;
}) => {
  return (
    <Flex alignItems="center">
      <Text fontSize={4} mb={2}>
        {title}
      </Text>
      <ButtonPencil onClick={setStep} disabled={disabled} mt={-6} />
    </Flex>
  );
};

export const List = styled.ul`
  padding-left: ${p => p.theme.space[4]}px;
  margin-bottom: ${p => p.theme.space[3]}px;
  margin-top: ${p => p.theme.space[3]}px;
  font-size: ${p => p.theme.fontSizes[1]}px;
`;
