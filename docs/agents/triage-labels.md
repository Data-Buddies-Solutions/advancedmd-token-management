# Triage Labels

The skills speak in terms of five canonical triage roles. This file maps those
roles to the intended label strings for this repository.

| Canonical role    | GitHub label       | Meaning                                |
| ----------------- | ------------------ | -------------------------------------- |
| `needs-triage`    | `needs-triage`     | Maintainer needs to evaluate the issue |
| `needs-info`      | `needs-info`       | Waiting for more information           |
| `ready-for-agent` | `ready-for-agent`  | Fully specified and ready for an agent |
| `ready-for-human` | `ready-for-human`  | Requires human implementation          |
| `wontfix`         | `wontfix`          | Will not be actioned                    |

Before applying a label, confirm it exists with `gh label list`. If a mapped
label is absent, report that it is unavailable; do not create it without
explicit approval.
