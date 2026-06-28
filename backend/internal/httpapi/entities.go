package httpapi

import "github.com/worldsignal/backend/internal/gql"

// registerEntityResolvers wires Phase B resolvers (articles, raw items,
// deliveries, jobs, analytics, …). Filled in incrementally.
func (s *Server) registerEntityResolvers(q, m map[string]gql.FieldResolver) {}
