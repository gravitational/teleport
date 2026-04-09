import { writeFile } from 'node:fs/promises';
import { dirname } from 'node:path';

const STIX_URL =
  'https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack.json';

interface StixExternalRef {
  source_name: string;
  external_id?: string;
  url?: string;
}

interface StixObject {
  type: string;
  name?: string;
  external_references?: StixExternalRef[];
  revoked?: boolean;
  x_mitre_deprecated?: boolean;
}

interface StixBundle {
  objects: StixObject[];
}

async function main() {
  const outPath =
    dirname(import.meta.url.replace('file://', '')) + '/mitre-lookup.json';

  process.stdout.write(`Fetching ATT&CK Enterprise STIX bundle…\n`);

  const res = await fetch(STIX_URL);
  if (!res.ok) throw new Error(`Failed to fetch: ${res.status}`);

  const bundle: StixBundle = await res.json();

  const lookup: Record<string, string> = {};
  let skipped = 0;

  for (const obj of bundle.objects) {
    if (obj.type !== 'attack-pattern') {
      continue;
    }

    if (obj.revoked || obj.x_mitre_deprecated) {
      skipped++;
      continue;
    }

    const ref = obj.external_references?.find(
      r => r.source_name === 'mitre-attack'
    );

    if (!ref?.external_id) {
      continue;
    }

    const id = ref.external_id;

    if (!obj.name) {
      process.stdout.write(`Warning: skipping ${id} because it has no name\n`);
      continue;
    }

    lookup[id] = obj.name ?? '';
  }

  const sorted = Object.fromEntries(
    Object.entries(lookup).sort(([a], [b]) => a.localeCompare(b))
  );

  await writeFile(outPath, JSON.stringify(sorted));

  process.stdout.write(
    `✔ Wrote ${Object.keys(sorted).length} techniques to ${outPath}\n`
  );
  process.stdout.write(`  (skipped ${skipped} revoked/deprecated)\n`);
}

main().catch(err => {
  process.stderr.write('\x1b[31mError:\x1b[0m ' + err.message + '\n');
  process.exit(1);
});
