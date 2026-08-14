import { Box, H2, LabelInput } from 'design';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import { pluralize } from 'shared/utils/text';

import {
  ReviewDayOfMonthOption,
  ReviewFrequencyOption,
  ReviewRecurrence,
} from 'e-teleport/AccessListManagement/Shared/Audit';
import { AccessListMember } from 'e-teleport/services/accessmanagement';

import { RoleAndTraitLabels } from '../Shared';
import { MembershipRequires } from './ReviewMembershipRequires';
import { EditButton, EditedRecurrence, List, ReviewStep } from './Shared';
import { getMembersDeleted } from './utils';

export function Summary({
  isOwner,
  setReviewStep,
  editedMembershipRequires,
  editedMembers,
  disabled,
  editedRecurrence,
  setEditedRecurrence,
  reviewNotes,
  setReviewNotes,
  originalMembers,
}: {
  isOwner: boolean;
  setReviewStep(r: ReviewStep): void;
  editedMembershipRequires: MembershipRequires;
  editedMembers: AccessListMember[];
  disabled: boolean;
  editedRecurrence: EditedRecurrence;
  setEditedRecurrence(e: EditedRecurrence): void;
  reviewNotes: string;
  setReviewNotes(s: string): void;
  originalMembers: AccessListMember[];
}) {
  return (
    <>
      <Box mb={4}>
        {isOwner ? (
          <H2 mb={2}>Membership Requirements (Read Only)</H2>
        ) : (
          <EditButton
            title="Membership Requirements"
            setStep={() => setReviewStep(ReviewStep.EditMembershipRequires)}
            disabled={disabled}
          />
        )}
        <RoleAndTraitLabels
          roles={editedMembershipRequires.roles}
          traits={editedMembershipRequires.traitLabels.map(
            l => `${l.name}: ${l.value}`
          )}
          accessKind="requirements"
          userKind="member"
        />
      </Box>
      <ReviewAudit
        isOwner={isOwner}
        disabled={disabled}
        editedMembers={editedMembers}
        editedMembershipRequires={editedMembershipRequires}
        editedRecurrence={editedRecurrence}
        setEditedRecurrence={setEditedRecurrence}
        reviewNotes={reviewNotes}
        setReviewNotes={setReviewNotes}
        originalMembers={originalMembers}
      />
    </>
  );
}

export function ReviewAudit({
  isOwner = false,
  disabled,
  editedMembers,
  editedMembershipRequires,
  editedRecurrence,
  setEditedRecurrence,
  reviewNotes,
  setReviewNotes,
  originalMembers,
}: {
  isOwner?: boolean;
  disabled: boolean;
  editedMembers: AccessListMember[];
  editedMembershipRequires: MembershipRequires;
  editedRecurrence: EditedRecurrence;
  setEditedRecurrence(e: EditedRecurrence): void;
  reviewNotes: string;
  setReviewNotes(s: string): void;
  originalMembers: AccessListMember[];
}) {
  const numMembersDeleted = getMembersDeleted(
    originalMembers,
    editedMembers
  ).length;

  return (
    <Box>
      <H2 mb={2}>Summary</H2>
      <List>
        <li>
          {getMemberApprovedMsg({
            numMembersApproved: originalMembers.length - numMembersDeleted,
            numRolesApproved: editedMembershipRequires.roles.length,
            numTraitsApproved: editedMembershipRequires.traitLabels.length,
          })}
        </li>
        <li>
          {numMembersDeleted} {pluralize(numMembersDeleted, 'member')} revoked
        </li>
      </List>
      {!isOwner && (
        <Box width="500px">
          <ReviewRecurrence
            isDisabled={disabled}
            onChangeFrequency={(o: ReviewFrequencyOption) =>
              setEditedRecurrence({ ...editedRecurrence, reviewFrequency: o })
            }
            onChangeDayOfMonth={(o: ReviewDayOfMonthOption) =>
              setEditedRecurrence({ ...editedRecurrence, reviewDayOfMonth: o })
            }
            selectedFrequency={editedRecurrence.reviewFrequency}
            selectedDayOfMonth={editedRecurrence.reviewDayOfMonth}
          />
        </Box>
      )}
      <LabelInput>Review Notes (Optional)</LabelInput>
      <FieldTextArea
        placeholder="Review Notes"
        value={reviewNotes}
        onChange={e => setReviewNotes(e.target.value)}
        resizable={true}
        readonly={disabled}
        textAreaCss={`
                height: 95px;
                width: 500px;
                `}
      />
    </Box>
  );
}

export function getMemberApprovedMsg({
  numMembersApproved,
  numRolesApproved,
  numTraitsApproved,
}: {
  numMembersApproved: number;
  numRolesApproved: number;
  numTraitsApproved: number;
}) {
  const memberPluralized = pluralize(numMembersApproved, 'member');
  const rolePluralized = pluralize(numRolesApproved, 'role');
  const traitPluralized = pluralize(numTraitsApproved, 'trait');

  let msg = `${numMembersApproved} ${memberPluralized} approved`;

  if (numRolesApproved > 0 && numTraitsApproved > 0) {
    return `${msg} to access ${numRolesApproved} ${rolePluralized} and ${numTraitsApproved} ${traitPluralized}`;
  } else if (numRolesApproved > 0) {
    return `${msg} to access ${numRolesApproved} ${rolePluralized}`;
  } else if (numTraitsApproved > 0) {
    return `${msg} to access ${numTraitsApproved} ${traitPluralized}`;
  }

  return msg;
}
