import { useState } from 'react';
import { useNavigate } from 'react-router';

import {
  Alert,
  Box,
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  H2,
} from 'design';
import { Cross, ShieldCheck } from 'design/Icon';
import { useToastNotifications } from 'shared/components/ToastNotification';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import {
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';
import cfg from 'e-teleport/config';
import {
  AccessList,
  AccessListMember,
  AccessListMemberKind,
  AccessListRequires,
  accessManagementService,
  ReviewAccessListRequest,
} from 'e-teleport/services/accessmanagement';
import { FeatureBox } from 'teleport/components/Layout';
import { Navigation } from 'teleport/components/Wizard/Navigation';

import { convertTraitLabelsToAllUserTraits } from '../../Traits';
import { RoleAndTraitLabels, type AccessListModified } from '../Shared';
import { ReviewMembers } from './ReviewMembers';
import {
  MembershipRequires,
  ReviewMembershipRequires,
} from './ReviewMembershipRequires';
import { EditedRecurrence, ReviewStep } from './Shared';
import { Summary } from './Summary';
import { getMembersDeleted } from './utils';

export const views = [
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
  reviewer,
  cancelReview,
  isOwner = false,
  isReadOnlyOktaList = false,
}: {
  accessList: AccessListModified;
  reviewer: string;
  cancelReview(): void;
  isOwner?: boolean;
  isReadOnlyOktaList?: boolean;
}) {
  const navigate = useNavigate();
  const [reviewStep, setReviewStep] = useState<ReviewStep>(views[0].step);
  const { attempt, run } = useAttempt('');
  const toastNotification = useToastNotifications();
  const { updateAccessListCache } = useAccessListManagementContext();

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

  const [editedMembers, setEditedMembers] = useState<
    AccessListModified['members'][number][]
  >(accessList.members);
  const [editedMembershipRequires, setEditedMembershipRequires] =
    useState<MembershipRequires>(accessList.membershipRequires);

  function reviewAccessList() {
    const review = getEditedAccessListFields({
      accessList,
      editedMembers,
      editedMembershipRequires,
      editedRecurrence,
    });
    run(() =>
      accessManagementService
        .reviewAccessList({
          ...review,
          name: accessList.id,
          notes: reviewNotes,
          reviewer,
        })
        .then(nextAuditDate => {
          // After review, route to access list listing.
          //
          // To handle the stale access list after review
          // (because of cache lag), modify this reviewed
          // access list fields where applicable
          // and send it with the router to manually update
          // the stale access list determined by difference
          // in audit.nextDate
          const reviewedAccessList: AccessList = accessList;
          reviewedAccessList.audit.nextDate = nextAuditDate;
          reviewedAccessList.members = editedMembers;

          const [newMembersCount, newMemberListCount] = editedMembers.reduce(
            (acc, m) => [
              acc[0] + (m.membershipKind === AccessListMemberKind.List ? 0 : 1),
              acc[1] + (m.membershipKind === AccessListMemberKind.List ? 1 : 0),
            ],
            [0, 0]
          );
          reviewedAccessList.membersCount = newMembersCount;
          reviewedAccessList.memberListCount = newMemberListCount;

          if (review.auditRecurrence) {
            reviewedAccessList.audit.recurrence = review.auditRecurrence;
          }

          toastNotification.add({
            severity: 'info',
            content: {
              title: `Submitted review for "${reviewedAccessList.title}"`,
              description: `Next review date is ${reviewedAccessList.audit.nextDate}`,
              icon: ShieldCheck,
            },
          });
          updateAccessListCache({
            mutationType: 'reviewed',
            accessList: reviewedAccessList,
          });
          navigate(cfg.getAccessListManagementRoute(), { replace: true });
        })
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
              title={'Cancel Review'}
              onClick={cancelReview}
              mr={2}
              ml={-2}
            >
              <Cross size="medium" color="text.main" />
            </ButtonIcon>
            <H1>Reviewing Access List: {accessList.title}</H1>
          </Flex>
          <Box mt={1} mb={3}>
            <Navigation currentStep={reviewStep} views={views} />
          </Box>
          {reviewStep === ReviewStep.EditMembershipRequires && (
            <Box mb={4} width="500px">
              {
                // TODO(kopiczko): That appears `isOwner` check broken when the adminWhoCanEdit is also an owner.
              }
              {isOwner ? (
                <>
                  <H2 mb={3}>Membership Requirements (Read Only)</H2>
                  <RoleAndTraitLabels
                    accessKind="requirements"
                    userKind="member"
                    roles={editedMembershipRequires.roles}
                    traits={editedMembershipRequires.traitLabels.map(
                      l => `${l.name}: ${l.value}`
                    )}
                  />
                </>
              ) : (
                <ReviewMembershipRequires
                  editedMembershipRequires={editedMembershipRequires}
                  setEditedMembershipRequires={setEditedMembershipRequires}
                />
              )}
            </Box>
          )}
          {reviewStep === ReviewStep.EditMembers && (
            <Box mb={4}>
              <ReviewMembers
                accessList={accessList}
                isReadOnlyOktaList={isReadOnlyOktaList}
                originalMembers={accessList.members}
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
          <Flex mt={6} mb={8}>
            <ButtonPrimary
              size="large"
              textTransform="none"
              onClick={() => handleNextButton(validator)}
              mr={3}
              disabled={attempt.status === 'processing'}
            >
              {views[reviewStep].buttonTitle}
            </ButtonPrimary>
            {reviewStep > 0 && (
              <ButtonSecondary
                size="large"
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
