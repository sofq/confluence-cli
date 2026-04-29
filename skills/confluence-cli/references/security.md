# Security Configuration

## Operation Policy (per profile)

Restrict which operations a profile can execute:

```json
{
  "profiles": {
    "agent": {
      "allowed_operations": ["pages get", "search *", "workflow *"]
    },
    "readonly": {
      "denied_operations": ["* delete*", "workflow *", "raw *"]
    }
  }
}
```

- Use `allowed_operations` OR `denied_operations`, not both
- Patterns use glob matching: `*` matches any sequence
- `allowed_operations`: implicit deny-all, only matching ops run
- `denied_operations`: implicit allow-all, only matching ops blocked

## Batch Limits

Default max batch size is 50. Override with `--max-batch N`.

## Audit Logging

Enable per-invocation with `--audit <path>`.
Logs NDJSON with: timestamp, user, profile, operation, exit code.
