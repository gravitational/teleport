import { useQuery } from '@tanstack/react-query';

import { fetchRecordings } from 'teleport/services/recordings/recordings';

// Beams live for at most 24 hours, so any recording belonging to a
// currently-listed beam falls within the last 24 hours. The recordings API has
// no server side per resource filter, so fetching this window and matching
// beams against the recorded hostnames client-side.
const RECORDINGS_WINDOW_MS = 24 * 60 * 60 * 1000;

// MAX_PAGES bounds the scan on clusters with an unusually high volume of
// session recordings. Each page holds up to 5000 recordings, and typical
// clusters produce fewer than that in 24 hours.
const MAX_PAGES = 5;

/**
 * useBeamRecordingHostnames returns the set of hostnames that have a session
 * recording within the last 24 hours. A beam's SSH sessions are recorded under
 * the hostname `beam-<beam.name>`, so a beam has recordings when that hostname
 * is present in the returned set. Used to decide which beam rows surface a
 * "session recordings" link.
 */
export function useBeamRecordingHostnames({
  clusterId,
  enabled,
}: {
  clusterId: string;
  enabled: boolean;
}) {
  return useQuery({
    enabled,
    queryKey: ['beam-recording-hostnames', clusterId],
    refetchOnWindowFocus: true,
    queryFn: async ({ signal }) => {
      const to = new Date();
      const from = new Date(to.getTime() - RECORDINGS_WINDOW_MS);
      const hostnames = new Set<string>();
      let startKey: string | undefined;
      let pages = 0;

      do {
        const page = await fetchRecordings(
          { clusterId, params: { from, to, startKey } },
          signal
        );
        for (const recording of page.recordings) {
          if (recording.hostname?.startsWith('beam-')) {
            hostnames.add(recording.hostname);
          }
        }
        startKey = page.startKey || undefined;
        pages += 1;
      } while (startKey && pages < MAX_PAGES);

      return hostnames;
    },
  });
}
