# Manual Windows Auth Signing Handoff

Use this runbook to collect the required fields and upload Microsoft's returned
ZIP and `submission.json` without using the helper tool.

Every field in `submission.json` comes from the sources below. Collect each at
the step shown, and record it in the release ticket as you go.

| Source | Fields |
|---|---|
| The CAB's `.metadata.json` (S3, written by CI) | `windowsauth_version`, `prepare_cab_s3_key`, `prepare_cab_sha256` |
| Your AWS session (`sts get-caller-identity`) | `cab_downloaded_by_aws_arn`, `uploaded_by_aws_arn` |
| Your clock (UTC) | `cab_downloaded_at`, `uploaded_at` |
| Microsoft's returned ZIP (local file) | `microsoft_submission.id`, `.returned_zip_filename`, `.returned_zip_sha256` |
| The ZIP upload to the signed bucket | `microsoft_submission.returned_zip_s3_key`, `.returned_zip_s3_version_id` |
| Fixed | `schema_version` = `1` (a number); `operator_note` = free text or `""` |

Do not take release values from GitHub job logs, the run summary, or Temporal.
Those only help you find the files. The metadata file is the source of truth.

## Step 1: find the CAB set

When the Generate CAB Release run succeeds, open the `windows-auth-package-cab`
run's summary. The **S3** and **Metadata** rows give the object locations, in
the form `s3://<unsigned bucket>/<origin>/<version>/<teleport-version>/<sha9>/teleport-auth-package-v<version>.cab`.

Three objects belong together: `<cab>`, `<cab>.sha256` and `<cab>.metadata.json`.

## Step 2: download them and record who and when

Log in with the download (Operator) profile, listed in the
[README](README.md#log-in).

1. `cab_downloaded_by_aws_arn`: before downloading, run
   `aws sts get-caller-identity --profile <download profile> --query Arn --output text`.
   Use the same profile for the download.
2. Download the three objects.
3. `cab_downloaded_at`: as soon as the download finishes, run `date -u +%Y-%m-%dT%H:%M:%SZ`.

Record both values now. The person who uploads later may be someone else, and
these values can't be recovered afterwards. If the helper did the download, the
same two values are in its `download.json`.

## Step 3: copy the release fields from `.metadata.json`

Open `<cab>.metadata.json` and copy these exactly:

| Field | Copy from | Notes |
|---|---|---|
| `windowsauth_version` | `.windowsauth_version` | Keep the leading `v`, e.g. `v1.0.1`. |
| `prepare_cab_s3_key` | `.cab.s3_key` | The key only: no `s3://` and no bucket name. |
| `prepare_cab_sha256` | `.signed_cab_sha256` | Cross-check: it equals the first word of `<cab>.sha256` and the run summary's SHA256 row. |

Also note **origin** and **key version**: the first two segments of
`.cab.s3_key` (e.g. `prod` and `1.0.1`). You need them in step 5.

A CAB without `.metadata.json` (built before the metadata change) cannot be
handed off this way; cut a new CAB release instead of reconstructing the fields.

## Step 4: submit the CAB and collect the ZIP fields

Submit only `<cab>` in Partner Center. Microsoft returns `Signed_<id>.zip`.
Don't rename it.

| Field | Where to get it |
|---|---|
| `microsoft_submission.id` | The text between `Signed_` and `.zip`. Check it matches the submission ID shown in Partner Center. Record it as a string, even if it's all digits. |
| `microsoft_submission.returned_zip_filename` | The ZIP's filename, with no folder, e.g. `Signed_13991973633690851.zip`. |
| `microsoft_submission.returned_zip_sha256` | `shasum -a 256 Signed_<id>.zip`: the first word, in lowercase. Hash the whole ZIP, not the DLL inside it. |

## Step 5: upload the ZIP and collect the upload fields

Log in with the upload (Admin) profile, listed in the
[README](README.md#log-in).

| Field | Where to get it |
|---|---|
| `microsoft_submission.returned_zip_s3_key` | `<origin>/<key version>/<returned_zip_filename>`, using origin and key version from step 3. This is the key you upload the ZIP to, in the signed bucket from `e/.environments/<env>/build/windows-signing.yaml`. |
| `uploaded_by_aws_arn` | `aws sts get-caller-identity --profile <upload profile> --query Arn --output text` |
| `microsoft_submission.returned_zip_s3_version_id` | The `VersionId` in the response of the ZIP upload (`aws s3api put-object … --checksum-algorithm CRC32`). In the console: the object's **Versions** tab, newest version. It must not be empty or `null`. If you upload again, use the new version's ID. |
| `uploaded_at` | `date -u +%Y-%m-%dT%H:%M:%SZ`, run as soon as the ZIP upload succeeds. |

## Step 6: fill in the fixed fields and publish

- `schema_version`: the number `1`.
- `operator_note`: optional text, or `""`.

Check every field against the [contract](README.md#submission-json-contract),
then upload `submission.json` to `<origin>/<key version>/submission.json` in the
signed bucket, with the upload profile, **after** the ZIP.

## Before you finish

- [ ] `prepare_cab_sha256` equals the `.sha256` file and the run summary.
- [ ] `returned_zip_s3_key` starts with the same `<origin>/<key version>/` as `prepare_cab_s3_key`.
- [ ] `returned_zip_s3_version_id` is the version from *your* upload.
- [ ] The two ARNs are the download and upload sessions, and `cab_downloaded_at` is earlier than `uploaded_at`.
