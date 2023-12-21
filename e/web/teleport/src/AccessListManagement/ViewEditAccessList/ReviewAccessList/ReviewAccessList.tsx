import React, { useState } from 'react';
import { useHistory } from 'react-router';
import useAttempt from 'shared/hooks/useAttemptNext';
import { Option } from 'shared/components/Select';
import {
  Box,
  Alert,
  Flex,
  Text,
  ButtonSecondary,
  ButtonIcon,
  ButtonPrimary,
} from 'design';
import { Cross } from 'design/Icon';
import Validation, { Validator } from 'shared/components/Validation';
import { FeatureBox } from 'teleport/components/Layout';
import { StepNavigation } from 'teleport/components/StepNavigation';

import {
  accessManagementService,
  AccessListMember,
  ReviewAccessListRequest,
  AccessListRequires,
} from 'e-teleport/services/accessmanagement';
import cfg from 'e-teleport/config';
import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';

import { convertTraitLabelsToAllUserTraits } from '../../Traits';
import { AccessListModified } from '../ViewEditAccessList';
import { RoleAndTraitLabels } from '../Shared';

import {
  MembershipRequires,
  ReviewMembershipRequires,
} from './ReviewMembershipRequires';
import { ReviewMembers } from './ReviewMembers';
import { Summary } from './Summary';
import { EditedRecurrence, ReviewStep } from './Shared';
import FinishedReview from './FinishedReview';
import { getMembersDeleted } from './utils';

export const reviewSteps = [
  {
    step: ReviewStep.EditMembershipRequires,
    title: 'Membership Requirements',
    buttonTitle: 'Approve Membership Requirements',
  },
  {
    step: ReviewStep.EditMembers,
    title: 'Members',
    buttonTitle: 'Approve Members',
  },
  { step: ReviewStep.Summary, title: 'Summary', buttonTitle: 'Submit Review' },
];

export function ReviewAccessList({
  accessList,
  roleOptions,
  reviewer,
  cancelReview,
  isOwner = false,
}: {
  accessList: AccessListModified;
  roleOptions: Option[];
  reviewer: string;
  cancelReview(): void;
  isOwner?: boolean;
}) {
  const history = useHistory();
  const [nextAuditDate, setNextAuditDate] = useState<Date | null>();
  const [reviewStep, setReviewStep] = useState<ReviewStep>(reviewSteps[0].step);
  const { attempt, run } = useAttempt('');

  const [reviewNotes, setReviewNotes] = useState('');
  const [editedRecurrence, setEditedRecurrence] = useState<EditedRecurrence>(
    () => {
      return {
        reviewDayOfMonth: getReviewDayOfMonthOption(
          accessList.audit.recurrence.dayOfMonth
        ),
        reviewFrequency: getReviewFrequencyOption(
          accessList.audit.recurrence.frequency
        ),
      };
    }
  );

  const [editedMembers, setEditedMembers] = useState<AccessListMember[]>(
    accessList.members
  );
  const [editedMembershipRequires, setEditedMembershipRequires] =
    useState<MembershipRequires>(accessList.membershipRequires);

  function reviewAccessList() {
    run(() =>
      accessManagementService
        .reviewAccessList({
          ...getEditedAccessListFields({
            accessList,
            editedMembers,
            editedMembershipRequires,
            editedRecurrence,
          }),
          name: accessList.id,
          notes: reviewNotes,
          reviewer,
        })
        .then(setNextAuditDate)
    );
  }

  function handleRemoveMember(member: AccessListMember) {
    const filtered = editedMembers.filter(m => m.name !== member.name);
    setEditedMembers(filtered);
  }

  function handleNextButton(validator: Validator) {
    if (reviewStep === ReviewStep.Summary) {
      reviewAccessList();
      return;
    }

    if (!validator.validate()) {
      return;
    }
    setReviewStep(prevStep => prevStep + 1);
  }

  return (
    <Validation>
      {({ validator }) => (
        <FeatureBox>
          <Flex alignItems="center" mb={2} mt={4}>
            <ButtonIcon
              alignItems="center"
              title={'Cancel Review'}
              onClick={cancelReview}
              mr={2}
              ml={-2}
            >
              <Cross size="medium" color="text.main" />
            </ButtonIcon>
            <Text bold fontSize={5}>
              Reviewing Access List: {accessList.title}
            </Text>
          </Flex>
          <Box mt={1} mb={3}>
            <StepNavigation currentStep={reviewStep} steps={reviewSteps} />
          </Box>
          {reviewStep === ReviewStep.EditMembershipRequires && (
            <Box mb={4} width="500px">
              {isOwner ? (
                <>
                  <Text fontSize={4} mb={3}>
                    Membership Requirements (Read Only)
                  </Text>
                  <RoleAndTraitLabels
                    roles={editedMembershipRequires.roles}
                    traits={editedMembershipRequires.traitLabels.map(
                      l => `${l.name}: ${l.value}`
                    )}
                  />
                </>
              ) : (
                <ReviewMembershipRequires
                  roleOptions={roleOptions}
                  editedMembershipRequires={editedMembershipRequires}
                  setEditedMembershipRequires={setEditedMembershipRequires}
                />
              )}
            </Box>
          )}
          {reviewStep === ReviewStep.EditMembers && (
            <Box mb={4}>
              <ReviewMembers
                editedMembers={editedMembers}
                onDeleteMember={handleRemoveMember}
              />
            </Box>
          )}
          {reviewStep === ReviewStep.Summary && (
            <>
              {attempt.status === 'failed' && (
                <Alert children={attempt.statusText} />
              )}
              <Summary
                setReviewStep={setReviewStep}
                editedMembershipRequires={editedMembershipRequires}
                editedMembers={editedMembers}
                disabled={attempt.status === 'processing'}
                editedRecurrence={editedRecurrence}
                setEditedRecurrence={setEditedRecurrence}
                reviewNotes={reviewNotes}
                setReviewNotes={setReviewNotes}
                originalMembers={accessList.members}
                isOwner={isOwner}
              />
            </>
          )}
          {nextAuditDate && (
            <FinishedReview
              nextAuditDate={nextAuditDate}
              onClick={() =>
                history.replace(
                  cfg.getAccessListManagementRoute(accessList.id),
                  {
                    reviewed: true,
                  }
                )
              }
            />
          )}
          <Flex mt={6} mb={8}>
            <ButtonPrimary
              textTransform="none"
              onClick={() => handleNextButton(validator)}
              mr={3}
              disabled={attempt.status === 'processing'}
            >
              {reviewSteps[reviewStep].buttonTitle}
            </ButtonPrimary>
            {reviewStep > 0 && (
              <ButtonSecondary
                textTransform="none"
                onClick={() => setReviewStep(prevStep => prevStep - 1)}
                disabled={attempt.status === 'processing'}
              >
                Back
              </ButtonSecondary>
            )}
          </Flex>
        </FeatureBox>
      )}
    </Validation>
  );
}

export function getEditedAccessListFields({
  accessList,
  editedMembers,
  editedMembershipRequires,
  editedRecurrence,
}: {
  accessList: AccessListModified;
  editedMembers: AccessListMember[];
  editedMembershipRequires: MembershipRequires;
  editedRecurrence: EditedRecurrence;
}): Omit<ReviewAccessListRequest, 'name' | 'reviewer' | 'notes'> {
  const membersDeleted = getMembersDeleted(accessList.members, editedMembers);

  const roles = {
    originals: accessList.membershipRequires.roles,
    edited: editedMembershipRequires.roles,
  };
  const changedRequiredRoles = isUpdated(roles);

  const traits = {
    originals: accessList.membershipRequires.traitLabels.map(
      t => `${t.name}${t.value}`
    ),
    edited: editedMembershipRequires.traitLabels.map(
      t => `${t.name}${t.value}`
    ),
  };
  const changedRequiredTraits = isUpdated(traits);

  // If `membershipRequires` object is null, it means
  // roles AND traits were not edited.
  //
  // If not null, means one OR both fields
  // have been edited.
  //
  // In the case of one field edited, the other
  // un-edited field also has to be included with the
  // un-edited values otherwise the backend interprets
  // it as "delete this field"
  let membershipRequires: AccessListRequires = null;
  if (changedRequiredRoles || changedRequiredTraits) {
    membershipRequires = {
      roles: editedMembershipRequires.roles,
      traits: convertTraitLabelsToAllUserTraits(
        editedMembershipRequires.traitLabels
      ),
    };
  }

  const changedReviewDayOfMonth =
    editedRecurrence.reviewDayOfMonth.value !==
    accessList.audit.recurrence.dayOfMonth;

  const changedReviewFrequency =
    editedRecurrence.reviewFrequency.value !==
    accessList.audit.recurrence.frequency;

  return {
    membersDeleted: membersDeleted.length > 0 ? membersDeleted : null,
    membershipRequires: membershipRequires,
    auditRecurrence: {
      frequency: changedReviewFrequency
        ? editedRecurrence.reviewFrequency.value
        : null,
      dayOfMonth: changedReviewDayOfMonth
        ? editedRecurrence.reviewDayOfMonth.value
        : null,
    },
  };
}

function isUpdated({
  originals,
  edited,
}: {
  originals: string[];
  edited: string[];
}) {
  const isSomeDeleted = !originals.every(original => edited.includes(original));
  const isSomeAdded = !edited.every(maybeNew => originals.includes(maybeNew));

  return isSomeDeleted || isSomeAdded;
}
