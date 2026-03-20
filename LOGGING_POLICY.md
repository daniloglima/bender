# Bender Logging Policy

This policy defines how logs should be emitted in Bender to keep observability useful without creating noise.

## Goals

- predictable message format
- low cardinality and low noise in production
- actionable error messages
- stable semantics for log levels

## Message Format

Each log line should follow:

- `Component.Method: message`

Examples:

- `Container.Resolve: Requesting instance for *MyService`
- `Container.resolve: Missing binding for *MyService, path: *Handler -> *UseCase`

The logger wrapper already adds timestamp and level metadata.

## Level Semantics

### `Error`

Use for failures that cause the current operation to fail.

Use cases:

- missing binding
- cycle detection
- provider creation failure
- provider execution failure

Rules:

- include key context (type, name, dependency path)
- avoid duplicate logging of the same error across stack layers

### `Info`

Use for lifecycle milestones and startup events.

Use cases:

- container creation summary
- module installation summary
- one-time initialization milestones

Rules:

- never log on every dependency resolution in hot paths

### `Debug`

Use for troubleshooting details and high-frequency flow events.

Use cases:

- begin/end of resolve
- cache hits
- waiting for in-flight singleton
- scope creation/disposal traces

Rules:

- safe to disable in production
- can include per-resolution traces

## Noise Control Rules

1. No duplicate error logging for the same failure path.
2. Hot-path operations should default to `Debug`, not `Info`.
3. Keep message text stable to help dashboards and grep-based workflows.
4. Avoid embedding user payloads, secrets, or high-cardinality raw blobs.

## Security Rules

- do not log credentials, tokens, secrets, or raw request bodies
- avoid personally identifiable data unless explicitly required and masked

## Review Checklist (for PRs)

- Does the new log line have the `Component.Method` prefix?
- Is the selected level correct for frequency and severity?
- Can the same failure be logged twice on the same path?
- Does the message include enough context to debug quickly?
- Could the message leak sensitive data?
