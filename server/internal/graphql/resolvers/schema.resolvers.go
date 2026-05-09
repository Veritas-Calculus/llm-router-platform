package resolvers

import "llm-router-platform/internal/graphql/generated"

// Resolver implementations are split by domain across this package.
// This placeholder keeps gqlgen's follow-schema resolver path occupied so it
// does not regenerate duplicate root Query/Mutation methods into this file.

type mutationResolver struct{ *Resolver }

type queryResolver struct{ *Resolver }

// Mutation returns the root mutation resolver.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns the root query resolver.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }
