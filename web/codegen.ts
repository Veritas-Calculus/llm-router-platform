import type { CodegenConfig } from '@graphql-codegen/cli';

/**
 * graphql-codegen produces strongly-typed TS hooks for every Query/Mutation
 * declared under src/lib/graphql/operations/*.ts. The audit (FE-H5) flagged
 * that 75 files disabled @typescript-eslint/no-explicit-any to cope with
 * useQuery<any>(THING); with the generated types Apollo v4 infers
 * <TData, TVars> automatically and schema changes surface as compile errors
 * at the call site instead of runtime undefined.
 *
 * Output lands in src/lib/graphql/generated/. The directory is gitignored
 * — `npm run codegen` produces it before lint/typecheck/build.
 *
 * The schema is read from the local server's .graphqls files so codegen
 * runs offline without needing a server endpoint. Schema changes require
 * re-running codegen.
 */
const config: CodegenConfig = {
  schema: '../server/internal/graphql/schema/*.graphqls',
  documents: ['src/lib/graphql/operations/**/*.ts'],
  generates: {
    // client-preset is the modern recommended setup. It produces:
    //   - generated/graphql.ts    schema-wide types
    //   - generated/gql.ts        graphql() helper that returns TypedDocumentNode
    //   - generated/fragment-masking.ts  optional fragment-masking helpers
    // Apollo's useQuery(graphql(`query X { … }`)) then infers TData/TVars
    // from the document — no <TData, TVars> annotation needed at call sites.
    'src/lib/graphql/generated/': {
      preset: 'client',
      config: {
        useTypeImports: true,
        scalars: {
          Money: {
            input: 'string | number',
            output: 'string',
          },
          // DateTime serializes as ISO-8601 over the wire; using `string`
          // means callers can pass it straight into `new Date(...)` without
          // an `as string` cast. Apollo 4.2 surfaces stronger types on
          // partial data which made the previous `unknown` fall through
          // as runtime-correct but type-system-wrong.
          DateTime: {
            input: 'string',
            output: 'string',
          },
        },
      },
      presetConfig: {
        // Skip __typename in the document writes — keeps existing operations
        // working without modification. (Apollo still adds it at the cache
        // layer for normalization.)
        skipTypename: false,
      },
    },
  },
  hooks: {
    afterAllFileWrite: ['echo "✓ graphql-codegen complete"'],
  },
};

export default config;
