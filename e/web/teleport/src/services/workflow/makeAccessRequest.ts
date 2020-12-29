import moment from 'moment';
import { AccessRequest } from './types';

export default function makeAccessRequest(json?: AccessRequest): AccessRequest {
  json = json || {
    id: '',
    state: '',
    user: '',
    expires: undefined,
    expiresDuration: '',
    created: undefined,
    createdDuration: '',
    roles: [],
    resolveReason: '',
    requestReason: '',
  };

  return {
    ...json,
    expiresDuration: json.expires ? getDurationText(json.expires) : '',
    createdDuration: json.created ? moment(json.created).fromNow() : '',
  };
}

function getDurationText(date: Date) {
  const duration = moment(new Date()).diff(date);
  return moment.duration(duration).humanize();
}
