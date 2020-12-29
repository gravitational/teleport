import { Store } from 'shared/libs/stores';
import { AccessRequest, makeAccessRequest } from 'e-teleport/services/workflow';

const ACCESS_REQUESTS_STORE = 'grv_teleport_access_requests_store';

type State = {
  waitingRoom: AccessRequest;
  assumed: Record<string, AccessRequest>;
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

  setWaitingRoom(request: AccessRequest) {
    this.setState({
      waitingRoom: request,
      assumed: {},
    });
  }

  // setApprovedWaitingRoom is used to update local storage
  // but prevent rerenders caused by setState notifying subscribers.
  // A usecase is when we want to reload a page in place of rerendering.
  setApprovedWaitingRoom(request: AccessRequest) {
    request.state = 'APPLIED';
    setLocalStorageState({
      waitingRoom: request,
      assumed: {
        [request.id]: request,
      },
    });
  }

  getWaitingRoom(): AccessRequest {
    return getLocalStorageState().waitingRoom;
  }

  addAssumed(request: AccessRequest) {
    this.state.assumed[request.id] = request;
    this.setState({
      waitingRoom: this.state.waitingRoom,
      assumed: this.state.assumed,
    });
  }

  getAssumed(): Record<string, AccessRequest> {
    return getLocalStorageState().assumed;
  }

  isAssumed(requestId: string) {
    return getLocalStorageState().assumed[requestId];
  }
}

function getLocalStorageState() {
  const item = window.localStorage.getItem(ACCESS_REQUESTS_STORE);
  return item
    ? JSON.parse(item)
    : {
        waitingRoom: makeAccessRequest(),
        assumed: {},
      };
}

function setLocalStorageState(state: State) {
  window.localStorage.setItem(ACCESS_REQUESTS_STORE, JSON.stringify(state));
}
