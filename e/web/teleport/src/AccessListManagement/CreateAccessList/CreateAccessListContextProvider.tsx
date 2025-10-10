import {
  createContext,
  FC,
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from 'react';
import { useHistory } from 'react-router-dom';

import { Validator } from 'shared/components/Validation';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import cfg from 'e-teleport/config';
import {
  AccessListMemberKind,
  AccessListType,
  accessManagementService,
  convertReviewFrequencyIntoBackendParsableValue,
  ReviewDayOfMonth,
  ReviewFrequency,
  UpsertAccessListRequest,
} from 'e-teleport/services/accessmanagement';
import useTeleport from 'teleport/useTeleport';

import { reviewDayOfMonthOpts, reviewFrequencyOpts } from '../Shared/Audit';
import { HybridUserOption } from '../Shared/Shared';
import { convertTraitLabelsToAllUserTraits } from '../Traits';
import { Grant, Members, Owners, Spec } from './types';

type State = {
  /**
   * If true, cluster cannot create anymore access lists.
   * Only true if license does not support IGS.
   */
  featureLimitReached: boolean;
  onCreate(validator: Validator): void;
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
};

const CreateAccessListContext = createContext<State>(null);

export const CreateAccessListContextProvider: FC<PropsWithChildren> = props => {
  const ctx = useTeleport();
  const history = useHistory();
  const { attempt: createAttempt, setAttempt: setCreateAttempt } =
    useAttempt('');
  const { updateAccessListCache } = useAccessListManagementContext();

  const perms = ctx.storeUser.getAccessListAccess();
  const canCreateAccessList = perms.create && perms.list && perms.read;
  const accessListCreator = ctx.storeUser.getUsername();

  const [featureLimitReached, setFeatureLimitReached] = useState(false);

  const [spec, setSpec] = useState<Spec>(() => defaultSpec);
  const [owners, setOwners] = useState<Owners>(defaultOwners);
  const [ownerGrant, setOwnerGrant] = useState<Grant>(defaultGrants);
  const [members, setMembers] = useState<Members>(defaultMembers);
  const [memberGrant, setMemberGrant] = useState<Grant>(defaultGrants);

  function reset() {
    setSpec(defaultSpec);
    setOwners(defaultOwners);
    setOwnerGrant(defaultGrants);
    setMembers(defaultMembers);
    setMemberGrant(defaultGrants);
  }

  // Check if this feature is allowed with current license entitlement.
  useEffect(() => {
    if (
      cfg.oss.entitlements.AccessLists.enabled &&
      !cfg.oss.entitlements.AccessLists.limit
    ) {
      return;
    }
    const limit = cfg.oss.entitlements.AccessLists.limit;

    function checkLimit() {
      accessManagementService
        .fetchAccessListsV2({ limit: limit + 5 })
        .then(resp => {
          if (resp.agents.length >= limit) {
            setFeatureLimitReached(true);
          }
        });
    }

    checkLimit();
  }, []);

  function onCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    // We don't need to setAttempt to "success"
    // since we are unmounting right after updating.
    setCreateAttempt({ status: 'processing' });

    const listToCreate = makeAccessListForRequest({
      spec,
      ownerGrant,
      owners,
      memberGrant,
      members,
      accessListCreator,
    });

    accessManagementService
      .createAccessList(listToCreate)
      // After creating, go back to access list listing.
      // Because of backend caching, we send the created list
      // as router state to be used to update the listing.
      .then(createdList => {
        // Add member counts to the created list in case they don't exist.
        if (!createdList.membersCount || !createdList.memberListCount) {
          const [membersCount, memberListCount] = getMemberCounts(listToCreate);
          createdList.membersCount = membersCount;
          createdList.memberListCount = memberListCount;
        }

        updateAccessListCache({
          mutationType: 'created',
          accessList: createdList,
        });
        history.replace(cfg.getAccessListManagementRoute());
      })
      .catch((e: Error) =>
        setCreateAttempt({ status: 'failed', statusText: e.message })
      );
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

function makeAccessListForRequest({
  spec,
  ownerGrant,
  owners,
  memberGrant,
  members,
  accessListCreator,
}: {
  spec: Spec;
  ownerGrant: Grant;
  owners: Owners;
  memberGrant: Grant;
  members: Members;
  accessListCreator: string;
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
    members: members.selectedMembers.map(m => {
      const { name, membership_kind } = getNameAndMembershipKind(m);
      return {
        name,
        joined: new Date(),
        added_by: accessListCreator,
        membership_kind,
      };
    }),
  };
}

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
