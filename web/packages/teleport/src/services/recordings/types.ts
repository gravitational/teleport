export type RecordingsQuery = {
  from: Date;
  to: Date;
  limit?: number;
  startKey?: string;
};

export type RecordingsResponse = {
  recordings: Recording[];
  startKey: string;
};

export type RecordingType = 'ssh' | 'desktop' | 'k8s';

export type Recording = {
  duration: number;
  durationText: string;
  sid: string;
  createdDate: Date;
  users: string;
  hostname: string;
  description: string;
  recordingType: RecordingType;
  playable: boolean;
};
