# Teleport `tctl` Command Output Schema

## `tctl acl summary --format=json`

Returns an array of access list objects. Each object has the following fields:

---

### Access List Object

| Field | Type | Nullable | Description |
|---|---|---|---|
| `name` | string | no | Internal UUID identifier for the access list. Used in `tctl acl reviews create <name>` and web UI URLs. |
| `title` | string | no | Human-readable display name. |
| `description` | string | no | Free-text description. May be empty string. |
| `next_audit_date` | string (RFC3339) | no | Date when the review is due. Compare against today to determine overdue status. |
| `owners` | Owner[] | no | Users responsible for managing this list. Can't be empty. |
| `grants` | Grants | no | Access granted to members of this list. May be empty. |
| `members` | Member[] | no | Current members of the list. May be empty. |
| `last_review` | LastReview | yes | Most recent review record. `null` if the list has never been reviewed. |

---

### Owner Object

| Field | Type | Nullable | Description |
|---|---|---|---|
| `name` | string | no | Username or email of the owner. |
| `membership_kind` | string (enum) | no | See Membership Kind enum below. |

---

### Grants Object

| Field | Type | Nullable | Description |
|---|---|---|---|
| `roles` | string[] | yes | Teleport roles granted to all members. `null` if none. Role names are free-form but may signal risk (e.g. `editor`, `admin` = high; `viewer`, `readonly` = low). |
| `traits` | object | yes | Key-value trait grants applied to members. `null` if none. |

---

### Member Object

| Field | Type | Nullable | Description |
|---|---|---|---|
| `name` | string | no | Username or email of the member. |
| `membership_kind` | string (enum) | no | See Membership Kind enum below. |
| `joined` | string (RFC3339) | no | Timestamp when the member was added to the list. |
| `reason` | string | yes | Free-text reason provided when the member was added. Absent if no reason was given. A reason mentioning "temporary", "short-term", or referencing a specific issue is a risk signal. |
| `expires` | string (RFC3339) | yes | Timestamp when the member's access expires. Absent if the membership does not expire. |
| `ineligible_status` | string (enum) | yes | Indicates why a member is ineligible. Absent if the member is currently eligible. See Ineligible Status enum below. |

---

### LastReview Object

| Field | Type | Nullable | Description |
|---|---|---|---|
| `review_date` | string (RFC3339) | no | Timestamp when the review was submitted. |
| `reviewers` | string[] | no | Usernames or emails of those who submitted the review. |
| `notes` | string | yes | Free-text notes recorded with the review. Absent if no notes were provided. |
| `removed_members` | string[] | yes | Usernames removed during this review. `null` if no members were removed. |

---

## Enums

### Membership Kind

| Value | Meaning |
|---|---|
| `MEMBERSHIP_KIND_USER` | Member is an individual Teleport user. |
| `MEMBERSHIP_KIND_LIST` | Member is another access list (nested membership). |

### Ineligible Status

| Value | Meaning | Risk Signal |
|---|---|---|
| `INELIGIBLE_STATUS_EXPIRED` | Member's `expires` timestamp is in the past. | Yes, member doesn't retain grants but should be removed. |
| `INELIGIBLE_STATUS_MISSING_REQUIREMENTS` | Member no longer meets the list's membership requirements (e.g. missing a required trait or role). | Yes, member doesn't retain grants but should be removed. |
| `INELIGIBLE_STATUS_USER_NOT_EXIST` | Member's Teleport user account no longer exists. | May be ephemeral SSO user, not a risk signal. |
