# Feature Status Guide

To maintain consistency across all specification documents, use the following 3 statuses.

## Status Criteria

| Status | Definition | Required characteristics |
|-------|----------|-----------------|
| `done` | The feature works end-to-end according to the contract (the CLI/API performs a real action). | No placeholders, the command invokes a real functional path, and state changes/actions can be verified. |
| `partial` | The feature can already be invoked and some behavior exists, but it is not end-to-end or still uses dummy data. | The command and handler are active, but the result is still simulated / limited / does not cover important cases. |
| `stub` | The feature has not been functionally implemented. | No real implementation; usually displays a `not implemented yet`, `TODO` message, or refuses with a placeholder error. |

## How to Use in Spec Files

- Use exactly one of these values: `done`, `partial`, `stub` (lowercase).
- Store the status in the specification column using these values; avoid mixing other terms like 'wip'/'coming soon'/'implemented'.
- If in doubt, default to `partial` when there is partial behavior and `stub` when there is no action at all.
