# Confluence v2 OpenAPI Spec — Known Gaps

Spec pinned: `spec/confluence-v2.json`
Source: `https://dac-static.atlassian.com/cloud/confluence/openapi-v2.v3.json`
Analyzed: 146 paths, 212 operations, 24 resource groups

## Gap 1: No Attachment Upload in v2 API

`POST /attachments` does not exist in the v2 spec. File upload remains v1-only:
`POST /wiki/rest/api/content/{id}/child/attachment`

**Workaround:** `cf raw POST /rest/api/content/{id}/child/attachment --body @file`

## Gap 2: Deprecated Operation

`GET /pages/{id}/children` (`getChildPages`) is marked deprecated in the spec.
It is generated but users should prefer the non-deprecated alternatives.

## Gap 3: EAP / Experimental Operations (18 ops)

These 18 operations carry `x-experimental: true` and/or `EAP` tag — they may
change without notice and are generated as-is from the spec:

`createSpace`, `getAvailableSpacePermissions`, `getAvailableSpaceRoles`,
`createSpaceRole`, `getSpaceRolesById`, `updateSpaceRole`, `deleteSpaceRole`,
`getSpaceRoleMode`, `getSpaceRoleAssignments`, `setSpaceRoleAssignments`,
`checkAccessByEmail`, `inviteByEmail`, `getDataPolicyMetadata`,
`getDataPolicySpaces`, `getForgeAppProperties`, `getForgeAppProperty`,
`putForgeAppProperty`, `deleteForgeAppProperty`

## Gap 4: Array Query Parameters Rendered as String Flags

Many list endpoints accept array-valued query parameters (e.g., `?id=1&id=2`).
The generator renders these as `--flag string` (single value only). Affected
parameters include: `status[]`, `id[]`, `space-id[]`, `label-id[]`, `prefix[]`
across resources including pages, blogposts, spaces, and attachments.

**Workaround:** Use `cf raw` with repeated query parameters, or pass
comma-separated values where the API accepts them.

## Gap 5: `embeds` Resource Undocumented in API Tag List

`embeds` appears as a path-first-segment resource with operations in the spec
but is not listed in the `tags` array in the spec root. It is generated as-is
since the spec is the source of truth. Treat as potentially internal/unstable.
