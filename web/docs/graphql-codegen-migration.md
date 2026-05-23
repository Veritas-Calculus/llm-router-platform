# graphql-codegen: per-call-site type migration guide

`npm run codegen` produces `src/lib/graphql/generated/` containing:

- `gql.ts` — the `graphql(...)` template helper that returns a `TypedDocumentNode<TData, TVars>` for any inline GraphQL string.
- `graphql.ts` — schema-wide types (every `Input`, every object, scalar wrappers).
- `index.ts` — re-exports.

The directory is gitignored. CI / `npm run build` re-generate it before TypeScript.

## Migration pattern

### Before (untyped)

```ts
/* eslint-disable @typescript-eslint/no-explicit-any */
import { useQuery } from '@apollo/client/react';
import { MY_USAGE_SUMMARY } from '@/lib/graphql/operations';

const { data } = useQuery<any>(MY_USAGE_SUMMARY, { variables: { orgId } });
const total = (data as any)?.myUsageSummary?.totalCost;
```

### After (typed)

```ts
import { useQuery } from '@apollo/client/react';
import { graphql } from '@/lib/graphql/generated';

const MyUsageSummaryDocument = graphql(`
  query MyUsageSummary($orgId: ID) {
    myUsageSummary(orgId: $orgId) { totalCost totalRequests totalTokens }
  }
`);

const { data } = useQuery(MyUsageSummaryDocument, { variables: { orgId } });
const total = data?.myUsageSummary?.totalCost;   // inferred as `number`
```

Apollo v4's `useQuery` infers `<TData, TVars>` from the `TypedDocumentNode` returned by `graphql(...)`. No `<any>`, no `as any` cast, and a schema change that drops `totalCost` becomes a compile error at this exact line.

## Incremental migration

The codebase has ~75 untyped call sites; do not rewrite them all at once. Two principles:

1. **New code uses `graphql()`.** Stop writing `gql\`…\`` constants in `operations/*.ts` for any new query — go straight to the `graphql()` helper at the call site.
2. **High-traffic / high-risk hooks first.** Auth (`Login`, `Me`, `RefreshToken`), webhooks, billing — anything that touches money or sessions benefits most from compile-time guarantees.

The existing `operations/*.ts` files keep working unchanged during the rollout; codegen happily produces typed wrappers around them.

## When the schema changes

After editing `server/internal/graphql/schema/*.graphqls`:

```bash
cd web
npm run codegen
```

(`npm run build` does this automatically.)

## Available scripts

| Script | What it does |
|---|---|
| `npm run codegen` | One-shot regeneration |
| `npm run codegen:watch` | Watch mode — re-emits on schema or operation changes |
| `npm run build` | Includes codegen as a build step (CI safe) |

## Why client-preset

Earlier configurations using `typescript` + `typescript-operations` + `typed-document-node` produced duplicate `Input` declarations (one with `Scalars['X']['input']` shape, another with raw `string | null`). The `@graphql-codegen/client-preset` bundle handles deduplication and is the recommended setup for Apollo / urql v3+. See `codegen.ts` for the configuration.
