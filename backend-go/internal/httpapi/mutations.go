package httpapi

import "github.com/worldsignal/backend/internal/gql"

// mutationResolvers wires GraphQL mutations. Implemented in Phase 2.
func (s *Server) mutationResolvers() map[string]gql.FieldResolver {
	return map[string]gql.FieldResolver{}
}
