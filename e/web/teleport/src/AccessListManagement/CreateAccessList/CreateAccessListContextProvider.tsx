import {
  createContext,
  FC,
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from 'react';
import { useLocation } from 'react-router';

import { Validator } from 'shared/components/Validation';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

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
import { AccessListWithPresetRequest } from 'e-teleport/services/accessmanagement/preset';
import useTeleport from 'teleport/useTeleport';

import {
  awsIcRoleAccessKind,
  newAccessRole,
  standardRoleAccessKind,
} from '../GuideEditor/Preset/role/role';
import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../Shared/Audit';
import { HybridUserOption } from '../Shared/Shared';
import { convertTraitLabelsToAllUserTraits } from '../Traits';
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
};

const CreateAccessListContext = createContext<State>(null);

export const CreateAccessListContextProvider: FC<
  PropsWithChildren & { mockCreatedAccessList?: AccessList }
> = props => {
  const loc = useLocation<ResumableAccessListState>();

  const ctx = useTeleport();

  const {
    attempt: createAttempt,
    setAttempt: setCreateAttempt,
    run: runAttempt,
  } = useAttempt('');
  const { updateAccessListCache, guideEditor } =
    useAccessListManagementContext();

  const { preset, standardRoleState, awsIcRoleState } = guideEditor;

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

  function reset() {
    setSpec(defaultSpec);
    setOwners(defaultOwners);
    setOwnerGrant(defaultGrants);
    setMembers(defaultMembers);
    setMemberGrant(defaultGrants);
    setCreateAttempt({ status: '' });
    setCreatedAccessList(undefined);

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

  function createDefaultAccessList(
    reqSpec: AccessListSpecForRequest,
    reqMembers: AccessListMembersForRequest
  ) {
    const listToCreate = { ...reqSpec, members: reqMembers };

    return runAttempt(() =>
      accessManagementService
        .createAccessList(listToCreate)
        .then(createdList => {
          updateCache(createdList, listToCreate);
        })
    );
  }

  function createAccessListWithPreset(
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
        metadata: { name: crypto.randomUUID(), labels: {}, revision: '' },
      },
      accessRoles,
    };

    return runAttempt(() =>
      accessManagementService
        .createAccessListWithPreset(req)
        .then(createdList => {
          updateCache(createdList, { ...reqSpec, members: reqMembers });
        })
    );
  }

  function onCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
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
        return createAccessListWithPreset(reqSpec, reqMembers);
      case '':
        return createDefaultAccessList(reqSpec, reqMembers);
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
  spec,
  ownerGrant,
  owners,
  memberGrant,
  members,
}: {
  spec: Spec;
  ownerGrant: Grant;
  owners: Owners;
  memberGrant: Grant;
  members: Members;
}) {
  return {
    // specs
    type: AccessListType.Default,
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
  traitsToGrant: [],
};
