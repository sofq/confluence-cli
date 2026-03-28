# Discovering Commands

`cf` has 242 commands, all auto-generated from the official Confluence OpenAPI v2 spec. Rather than memorizing them, use `cf schema` to explore what is available.

## Four discovery modes

**1. Resource-to-verb mapping** (default, best starting point):
```bash
cf schema
# Shows every resource and its available verbs
```

**2. List all resource names:**
```bash
cf schema --list
# pages, spaces, search, workflow, diff, export, blogposts, ...
```

**3. All operations for a resource:**
```bash
cf schema pages
# Lists every operation under the "pages" resource, with flags
```

**4. Full schema for a single operation:**
```bash
cf schema pages get
# Shows all available flags, types, and descriptions for "pages get"
```

::: tip
Start with `cf schema` or `cf schema --list` to orient yourself, then drill into a specific resource and operation. This is especially useful for AI agents that need to discover commands at runtime.
:::
