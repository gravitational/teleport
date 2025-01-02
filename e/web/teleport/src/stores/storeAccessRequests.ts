import { Store } from 'shared/libs/stores';
import {
  AccessRequest,
  makeAccessRequest,
} from 'shared/services/accessRequests';

const ACCESS_REQUESTS_STORE = 'grv_teleport_access_requests_store';

type State = {
  waitingRoom: AccessRequest;
  assumed: Record<string, AccessRequest>;
  // sessionExpiry is the absolute time the current web session expires
  // for the most recently consumed access request.
  sessionExpiry: Date;
};

export default class StoreAccessRequests extends Store<State> {
  constructor() {
    super();
    this.state = getLocalStorageState();
  }

  // Override setState method from Store.
  // Calling this method will notify all subscribers.
  setState(nextState: State) {
    setLocalStorageState(nextState);
    super.setState(nextState);
  }

  // setWaitingRoom is used to update non-approved requests (denied, still pending, error),
  // which requires re-rendering of the component.
  setWaitingRoom(request: AccessRequest) {
    this.setState({
      waitingRoom: request,
      assumed: {},
      sessionExpiry: null,
    });
  }

  // setApprovedWaitingRoom is used to update state with requests that were approved.
  // Approved requests requires page reload to apply new perms. This method prevents
  // unncessary rerendering right before a page reload.
  setApprovedWaitingRoom(request: AccessRequest, expires: Date) {
    request.state = 'APPLIED';
    setLocalStorageState({
      waitingRoom: request,
      assumed: {
        [request.id]: request,
      },
      sessionExpiry: expires,
    });
  }

  getWaitingRoom(): AccessRequest {
    return getLocalStorageState().waitingRoom;
  }

  addAssumed(request: AccessRequest, expires: Date) {
    this.state.assumed[request.id] = request;
    this.setState({
      waitingRoom: this.state.waitingRoom,
      assumed: this.state.assumed,
      sessionExpiry: expires,
    });
  }

  getAssumed(): Record<string, AccessRequest> {
    return getLocalStorageState().assumed;
  }

  isAssumed(requestId: string) {
    return getLocalStorageState().assumed[requestId];
  }

  // getAssumedRoles returns a list of all the roles the user is assigned.
  getAssumedRoles(): string[] {
    const assumed = this.getAssumed();
    let roles: string[] = [];

    Object.keys(assumed).forEach(key => {
      const request = assumed[key];
      const newRoles = request.roles.filter(r => !roles.includes(r));
      roles = [...roles, ...newRoles];
    });

    return roles;
  }

  getSessionExpiry() {
    return getLocalStorageState().sessionExpiry;
  }

  clearAssumes() {
    setLocalStorageState({
      waitingRoom: this.state.waitingRoom,
      assumed: {},
      sessionExpiry: null,
    });
  }
}

function getLocalStorageState() {
  const item = window.localStorage.getItem(ACCESS_REQUESTS_STORE);
  return item
    ? JSON.parse(item)
    : {
        waitingRoom: makeAccessRequest(),
        assumed: {},
        sessionExpiry: null,
      };
}

function setLocalStorageState(state: State) {
  window.localStorage.setItem(ACCESS_REQUESTS_STORE, JSON.stringify(state));
}
