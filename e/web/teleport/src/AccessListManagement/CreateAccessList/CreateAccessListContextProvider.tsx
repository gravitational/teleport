import {
  createContext,
  FC,
  PropsWithChildren,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { useLocation, type Location } from 'react-router';

import { Validator } from 'shared/components/Validation';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { ensureError } from 'shared/utils/error';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import {
  AccessList,
  AccessListMemberKind,
  AccessListType,
  accessManagementService,
  convertReviewFrequencyIntoBackendParsableValue,
  ReviewDayOfMonth,
  ReviewFrequency,
  UpsertAccessListRequest,
} from 'e-teleport/services/accessmanagement';
import {
  AccessListBodyRequest,
  AccessListWithPresetRequest,
} from 'e-teleport/services/accessmanagement/preset';
import auth from 'teleport/services/auth';
import { MfaChallengeScope } from 'teleport/services/auth/auth';
import { MfaChallengeResponse } from 'teleport/services/mfa';
import {
  AccessListEvent,
  AccessListStepStatusEvent,
} from 'teleport/services/userEvent/accessListEvents';
import useTeleport from 'teleport/useTeleport';

import {
  awsIcRoleAccessKind,
  newAccessRole,
  standardRoleAccessKind,
} from '../GuideEditor/Preset/role/role';
import { getTerraformBlockCommentWithRole } from '../GuideEditor/Terraform/terraform';
import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../Shared/Audit';
import { HybridUserOption } from '../Shared/Shared';
import { convertTraitLabelsToAllUserTraits, TraitLabel } from '../Traits';
import {
  cleanupPresetAcl,
  PendingCleanup,
  tryPresetAclCleanup,
} from './presetcleanup';
import { ResumableAccessListState } from './route';
import { Grant, Members, Owners, Spec } from './types';

type State = {
  /**
   * If true, cluster cannot create anymore access lists.
   * Only true if license does not support IGS.
   */
  featureLimitReached: boolean;
  onCreate(validator: Validator): Promise<boolean>;
  createAttempt: Attempt;
  setCreateAttempt(attempt: Attempt): void;
  owners: Owners;
  setOwners(owners: Owners): void;
  members: Members;
  setMembers(members: Members): void;
  spec: Spec;
  setSpec(spec: Spec): void;
  memberGrant: Grant;
  setMemberGrant(grant: Grant): void;
  ownerGrant: Grant;
  setOwnerGrant(grant: Grant): void;
  /**
   * True if user meets RBAC requirement to
   * create an access list.
   */
  canCreateAccessList: boolean;
  reset(): void;
  /**
   * Only set after a successful call to onCreate func.
   */
  createdAccessList: AccessList;
  getResumableAccessListState(): ResumableAccessListState;
  /**
   * The UUID generated for the new access list being created.
   */
  newAccessListId: string;
  requiresCleanup?: PendingCleanup;
  dismissCleanupError(): void;
};

const CreateAccessListContext = createContext<State>(null);

export const CreateAccessListContextProvider: FC<
  PropsWithChildren & { mockCreatedAccessList?: AccessList }
> = props => {
  const loc = useLocation() as Location<ResumableAccessListState>;

  const ctx = useTeleport();

  const {
    attempt: createAttempt,
    setAttempt: setCreateAttempt,
    run: runAttempt,
  } = useAttempt('');
  const { updateAccessListCache, guideEditor } =
    useAccessListManagementContext();

  // This safeguards against a possible (though highly unlikely) case of
  // rapid clicks starting multiple create processes that can trigger the
  // auto cleanup by accident, which can undo a successfully created access list.
  // The auto cleanup gets triggered by any error, including "already exists".
  const createInFlightRef = useRef(false);

  const { preset, standardRoleState, awsIcRoleState, emitEvent, terraform } =
    guideEditor;

  const perms = ctx.storeUser.getAccessListAccess();
  const canCreateAccessList = perms.create && perms.list && perms.read;
  const accessListCreator = ctx.storeUser.getUsername();

  const [featureLimitReached, setFeatureLimitReached] = useState(false);

  const [spec, setSpec] = useState<Spec>(() => loc.state?.spec ?? defaultSpec);

  const [owners, setOwners] = useState<Owners>(
    () => loc.state?.owners ?? defaultOwners
  );
  const [ownerGrant, setOwnerGrant] = useState<Grant>(
    () => loc.state?.ownerGrant ?? defaultGrants
  );

  const [members, setMembers] = useState<Members>(
    () => loc.state?.members ?? defaultMembers
  );
  const [memberGrant, setMemberGrant] = useState<Grant>(
    () => loc.state?.memberGrant ?? defaultGrants
  );

  const [createdAccessList, setCreatedAccessList] = useState<AccessList>(
    props.mockCreatedAccessList
  );

  const [newAccessListId, setNewAccessListId] = useState(() =>
    crypto.randomUUID()
  );

  // requiresCleanup defines which resources require cleaning up (delete):
  const [requiresCleanup, setRequiresCleanup] = useState<PendingCleanup>();

  // This effect triggers the terraform configs to be regenerated when state
  // changes. It's debounced to reduce the number of api calls.
  useEffect(() => {
    if (preset !== 'long-term' && preset !== 'short-term') {
      return;
    }

    // Skip calling the API if a user has just started and
    // has not defined any access yet.
    if (
      !terraform.hasMutatedConfig &&
      !awsIcRoleState.definedAccess() &&
      !standardRoleState.hasAnyAccessDefined() &&
      !spec.title
    ) {
      return;
    }

    // Skip if any owner/member trait has an empty key or value.
    if (
      hasEmptyTrait(ownerGrant.traitsToGrant) ||
      hasEmptyTrait(memberGrant.traitsToGrant) ||
      hasEmptyTrait(owners.traitLabels) ||
      hasEmptyTrait(members.traitLabels)
    ) {
      return;
    }

    const accessRoles = [];
    if (standardRoleState.hasAnyAccessDefined()) {
      accessRoles.push(
        getTerraformBlockCommentWithRole(
          'standard',
          newAccessRole({
            kind: standardRoleAccessKind,
            roleConditions: standardRoleState.roleConditions,
            withoutOptions: true,
          })
        )
      );
    }

    if (awsIcRoleState.definedAccess()) {
      accessRoles.push(
        getTerraformBlockCommentWithRole(
          'awsic',
          newAccessRole({
            kind: awsIcRoleAccessKind,
            roleConditions: awsIcRoleState.roleConditions,
            withoutOptions: true,
          })
        )
      );
    }

    let accessList: AccessListBodyRequest;
    if (spec.title) {
      accessList = {
        spec: makeAccessListSpecForRequest({
          // To manage members with terraform, type static is required
          type: AccessListType.Static,
          spec,
          ownerGrant,
          owners,
          memberGrant,
          members,
        }),
        members: makeAccessListMembersForTerraformRequest({
          members,
        }),
        metadata: { name: newAccessListId, labels: {}, revision: '' },
      };
    }

    terraform.regenerateConfig({
      accessRoles,
      accessListId: newAccessListId,
      accessList,
      presetType: preset,
    });
  }, [
    awsIcRoleState.roleConditions,
    standardRoleState.roleConditions,
    terraform.hasMutatedConfig,
    spec,
    ownerGrant,
    owners,
    memberGrant,
    members,
    preset,
    newAccessListId,
  ]);

  function reset() {
    createInFlightRef.current = false;
    setRequiresCleanup(undefined);
    setSpec(defaultSpec);
    setOwners(defaultOwners);
    setOwnerGrant(defaultGrants);
    setMembers(defaultMembers);
    setMemberGrant(defaultGrants);
    setCreateAttempt({ status: '' });
    setCreatedAccessList(undefined);
    setNewAccessListId(crypto.randomUUID());

    checkFeatureLimit();
  }

  function getResumableAccessListState(): ResumableAccessListState {
    return {
      spec,
      owners,
      ownerGrant,
      members,
      memberGrant,
    };
  }

  function checkFeatureLimit() {
    if (
      cfg.oss.entitlements.AccessLists.enabled &&
      !cfg.oss.entitlements.AccessLists.limit
    ) {
      return;
    }
    const limit = cfg.oss.entitlements.AccessLists.limit;

    accessManagementService
      .fetchAccessListsV2({ limit: limit + 5 })
      .then(resp => {
        if (resp.agents.length >= limit) {
          setFeatureLimitReached(true);
        }
      });
  }

  // Check if this feature is allowed with current license entitlement.
  useEffect(() => {
    checkFeatureLimit();
  }, []);

  function updateCache(
    createdList: AccessList,
    listCreated: AccessListSpecForRequest & {
      members: AccessListMembersForRequest;
    }
  ) {
    // Add member counts to the created list in case they don't exist.
    if (!createdList.membersCount || !createdList.memberListCount) {
      const [membersCount, memberListsCount] = getMemberCounts(listCreated);
      createdList.membersCount = membersCount;
      createdList.memberListCount = memberListsCount;
    }

    setCreatedAccessList(createdList);

    updateAccessListCache({
      mutationType: 'created',
      accessList: createdList,
    });
  }

  function createCustomAccessList(
    reqSpec: AccessListSpecForRequest,
    reqMembers: AccessListMembersForRequest
  ) {
    const listToCreate = { ...reqSpec, members: reqMembers };

    return runAttempt(() =>
      accessManagementService
        .createAccessList(listToCreate)
        .then(createdList => {
          updateCache(createdList, listToCreate);
          emitEvent({
            event: AccessListEvent.Completed,
            stepStatus: AccessListStepStatusEvent.Success,
          });
        })
        .catch((err: Error) => {
          emitEvent({
            event: AccessListEvent.Completed,
            stepStatus: AccessListStepStatusEvent.Error,
            stepStatusError: err.message,
          });
          throw err;
        })
    );
  }

  async function createAccessListWithPreset(
    reqSpec: AccessListSpecForRequest,
    reqMembers: AccessListMembersForRequest
  ) {
    const accessRoles = [];

    if (standardRoleState.hasAnyAccessDefined()) {
      accessRoles.push(
        newAccessRole({
          kind: standardRoleAccessKind,
          roleConditions: standardRoleState.roleConditions,
        })
      );
    }

    if (awsIcRoleState.definedAccess()) {
      accessRoles.push(
        newAccessRole({
          kind: awsIcRoleAccessKind,
          roleConditions: awsIcRoleState.roleConditions,
        })
      );
    }

    const req: AccessListWithPresetRequest = {
      presetType: preset,
      accessList: {
        spec: reqSpec,
        members: reqMembers,
        metadata: { name: newAccessListId, labels: {}, revision: '' },
      },
      accessRoles,
    };

    // Get reusable mfa response, so users aren't getting asked
    // to re-authn multiple times in a row.
    let mfaResponse: MfaChallengeResponse;
    try {
      setCreateAttempt({ status: 'processing' });
      const challenge = await auth.getMfaChallenge({
        scope: MfaChallengeScope.ADMIN_ACTION,
        allowReuse: true,
        isMfaRequiredRequest: {
          admin_action: {},
        },
      });
      mfaResponse = await auth.getMfaChallengeResponse(challenge);

      const stillRequiresCleanup = await tryPresetAclCleanup(
        newAccessListId,
        requiresCleanup,
        mfaResponse
      );
      setRequiresCleanup(stillRequiresCleanup || undefined);
      if (stillRequiresCleanup) {
        // Can't proceed if not all resources from previous attempt is not
        // cleaned upped.
        setCreateAttempt({ status: '' });
        return false;
      }
    } catch (err) {
      const ensuredError = ensureError(err);
      setCreateAttempt({ status: 'failed', statusText: ensuredError.message });
      return false;
    }

    return runAttempt(async () => {
      try {
        const createdList =
          await accessManagementService.createAccessListWithPreset(
            req,
            mfaResponse
          );

        updateCache(createdList, { ...reqSpec, members: reqMembers });
        emitEvent({
          event: AccessListEvent.Completed,
          stepStatus: AccessListStepStatusEvent.Success,
        });
      } catch (err) {
        const ensuredError = ensureError(err);
        emitEvent({
          event: AccessListEvent.Completed,
          stepStatus: AccessListStepStatusEvent.Error,
          stepStatusError: ensuredError.message,
        });
        await cleanupPresetAcl(
          newAccessListId,
          ensuredError,
          mfaResponse,
          setRequiresCleanup
        );
      }
    });
  }

  function dismissCleanupError() {
    setRequiresCleanup(
      requiresCleanup ? { ...requiresCleanup, error: undefined } : undefined
    );
  }

  function onCreate(validator: Validator) {
    const isPresetCreate = preset === 'long-term' || preset === 'short-term';
    if (isPresetCreate) {
      if (createInFlightRef.current) {
        return Promise.resolve(false);
      }
      createInFlightRef.current = true;
    }

    if (!validator.validate()) {
      createInFlightRef.current = false;
      return Promise.resolve(false);
    }

    const reqSpec = makeAccessListSpecForRequest({
      spec,
      ownerGrant,
      owners,
      memberGrant,
      members,
    });

    const reqMembers = makeAccessListMembersForRequest({
      members,
      accessListCreator,
    });

    switch (preset) {
      case 'long-term':
      case 'short-term':
        return createAccessListWithPreset(reqSpec, reqMembers).finally(() => {
          createInFlightRef.current = false;
        });
      case '':
        return createCustomAccessList(reqSpec, reqMembers);
      default:
        preset satisfies never;
    }
  }

  return (
    <CreateAccessListContext.Provider
      value={{
        featureLimitReached,
        createAttempt,
        setCreateAttempt,
        owners,
        setOwners,
        members,
        setMembers,
        spec,
        setSpec,
        memberGrant,
        setMemberGrant,
        ownerGrant,
        setOwnerGrant,
        onCreate,
        canCreateAccessList,
        reset,
        createdAccessList,
        getResumableAccessListState,
        newAccessListId,
        requiresCleanup,
        dismissCleanupError,
      }}
    >
      {props.children}
    </CreateAccessListContext.Provider>
  );
};

export const useCreateAccessList = () => {
  const context = useContext(CreateAccessListContext);

  if (!context) {
    throw new Error(
      'useCreateAccessList requires CreateAccessListContextProvider context.'
    );
  }

  return context;
};

/**
 * getMemberCounts gets the count of members as users and
 * members as an access list (nested).
 * @returns [member count as users, member count as nested access list]
 */
function getMemberCounts(accessList: UpsertAccessListRequest) {
  return accessList.members.reduce(
    (acc, m) => [
      acc[0] + (m.membership_kind === AccessListMemberKind.List ? 0 : 1),
      acc[1] + (m.membership_kind === AccessListMemberKind.List ? 1 : 0),
    ],
    [0, 0]
  );
}

/**
 * Gets the name of an owner or a member and their
 * membership kind.
 */
function getNameAndMembershipKind(o: HybridUserOption) {
  if (typeof o.value !== 'object') {
    return { name: o.value, membership_kind: AccessListMemberKind.User };
  }

  return {
    name: o.value.name,
    membership_kind:
      'membershipKind' in o.value
        ? o.value.membershipKind
        : AccessListMemberKind.User,
  };
}

function makeAccessListSpecForRequest({
  type = AccessListType.Default,
  spec,
  ownerGrant,
  owners,
  memberGrant,
  members,
}: {
  type?: AccessListType;
  spec: Spec;
  ownerGrant: Grant;
  owners: Owners;
  memberGrant: Grant;
  members: Members;
}) {
  return {
    // specs
    type,
    title: spec.title,
    description: spec.description,
    audit: {
      recurrence: {
        frequency: convertReviewFrequencyIntoBackendParsableValue(
          spec.reviewFrequency.value
        ),
        day_of_month: spec.reviewDayOfMonth.value,
      },
      next_audit_date: spec.auditStartDate,
    },
    // owners
    owner_grants: {
      roles: ownerGrant.rolesToGrant.map(r => r.value),
      traits: convertTraitLabelsToAllUserTraits(ownerGrant.traitsToGrant),
      scoped_roles: ownerGrant.scopedRolesToGrant,
    },
    ownership_requires: {
      roles: owners.selectedRolesRequired.map(r => r.value),
      traits: convertTraitLabelsToAllUserTraits(owners.traitLabels),
    },
    owners: owners.selectedOwners.map(o => getNameAndMembershipKind(o)),
    // members
    grants: {
      roles: memberGrant.rolesToGrant.map(r => r.value),
      traits: convertTraitLabelsToAllUserTraits(memberGrant.traitsToGrant),
      scoped_roles: memberGrant.scopedRolesToGrant,
    },
    membership_requires: {
      roles: members.selectedRolesRequired.map(r => r.value),
      traits: convertTraitLabelsToAllUserTraits(members.traitLabels),
    },
  };
}

function makeAccessListMembersForRequest({
  members,
  accessListCreator,
}: {
  members: Members;
  accessListCreator: string;
}) {
  return members.selectedMembers.map(m => {
    const { name, membership_kind } = getNameAndMembershipKind(m);
    return {
      name,
      joined: new Date(),
      added_by: accessListCreator,
      membership_kind,
    };
  });
}

function makeAccessListMembersForTerraformRequest({
  members,
}: {
  members: Members;
}) {
  return members.selectedMembers.map(m => {
    const { name, membership_kind } = getNameAndMembershipKind(m);
    return {
      name,
      membership_kind,
    };
  });
}

type AccessListSpecForRequest = ReturnType<typeof makeAccessListSpecForRequest>;

type AccessListMembersForRequest = ReturnType<
  typeof makeAccessListMembersForRequest
>;

const defaultOwners: Owners = {
  selectedRolesRequired: [],
  eligibleOwners: [],
  selectedOwners: [],
  traitLabels: [],
  traitLookup: {},
};

const defaultMembers: Members = {
  selectedRolesRequired: [],
  eligibleMembers: [],
  selectedMembers: [],
  traitLabels: [],
  traitLookup: {},
};

const defaultSpec: Spec = {
  title: '',
  description: '',
  // Default first day of month.
  reviewDayOfMonth: reviewDayOfMonthOpts.find(o => {
    return o.value === ReviewDayOfMonth.FirstDayOfMonth;
  }),
  // Default to 6 months.
  reviewFrequency: reviewFrequencyOpts.find(
    o => o.value === ReviewFrequency.SixMonths
  ),
  auditStartDate: null,
};

const defaultGrants: Grant = {
  rolesToGrant: [],
  scopedRolesToGrant: [],
  traitsToGrant: [],
};

function hasEmptyTrait(traits: TraitLabel[]): boolean {
  return traits.some(t => !t.name || !t.value);
}
