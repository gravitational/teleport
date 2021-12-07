import moment from 'moment';
import { Recording } from './types';

export default function makeRecording({
  participants = [],
  time,
  session_start,
  session_stop,
  server_hostname,
  interactive,
  session_recording = 'on',
  sid,
  proto = '',
  kubernetes_cluster = '',
  kubernetes_pod_namespace = '',
  kubernetes_pod_name = '',
}): Recording {
  let durationText = '';
  let duration = 0;
  if (session_start && session_stop) {
    duration = moment(session_stop).diff(session_start);
    durationText = moment.duration(duration).humanize();
  }

  let hostname = server_hostname || 'N/A';
  // For Kubernetes sessions, put the full pod name as 'hostname'.
  if (proto === 'kube') {
    hostname = `${kubernetes_cluster}/${kubernetes_pod_namespace}/${kubernetes_pod_name}`;
  }

  // Description set to play for interactive so users can search by "play".
  let description = interactive ? 'play' : 'non-interactive';
  if (session_recording === 'off') {
    description = 'recording disabled';
  }

  return {
    duration,
    durationText,
    sid,
    createdDate: time,
    users: participants.join(', '),
    hostname,
    description,
  };
}
