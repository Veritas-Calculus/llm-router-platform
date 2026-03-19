// Package gqlhandler provides the GraphQL HTTP handler for the Gin router.
package gqlhandler

import (
	"context"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"go.uber.org/zap"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/generated"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/graphql/resolvers"
)

// ─── Prometheus Metrics ─────────────────────────────────────────────

var (
	graphqlRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "requests_total",
			Help:      "Total number of GraphQL operations.",
		},
		[]string{"operation", "status"},
	)

	graphqlDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "duration_seconds",
			Help:      "GraphQL operation duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	graphqlComplexityRejected = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "complexity_rejected_total",
			Help:      "Number of queries rejected due to complexity limits.",
		},
	)

	graphqlDepthRejected = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "depth_rejected_total",
			Help:      "Number of queries rejected due to depth limits.",
		},
	)
)

// Handler wraps the gqlgen server with Gin integration.
type Handler struct {
	server *handler.Server
	logger *zap.Logger
}

// NewHandler creates a new GraphQL handler with all security controls.
func NewHandler(resolver *resolvers.Resolver, cfg *config.Config, logger *zap.Logger) *Handler {
	// Wire directives to generated config
	c := generated.Config{Resolvers: resolver}
	c.Directives.Auth = func(
		ctx context.Context, obj interface{}, next graphql.Resolver, role *model.Role,
	) (interface{}, error) {
		return directives.Auth(ctx, obj, next, role)
	}
	c.Directives.RateLimit = func(
		ctx context.Context, obj interface{}, next graphql.Resolver, max int, window string,
	) (interface{}, error) {
		return directives.RateLimit(ctx, obj, next, max, window)
	}

	srv := handler.New(generated.NewExecutableSchema(c))

	// ── Transports ──
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.POST{})

	// ── Query Complexity Limiting ──
	// Lists cost 10x, max complexity 200
	srv.Use(extension.FixedComplexityLimit(200))

	// ── Introspection ──
	if cfg.Server.Mode != "release" {
		srv.Use(extension.Introspection{})
	}

	// ── Query Depth Limiting ──
	const maxDepth = 7
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		if oc.Operation != nil {
			depth := selectionDepth(oc.Operation.SelectionSet)
			if depth > maxDepth {
				graphqlDepthRejected.Inc()
				return graphql.OneShot(graphql.ErrorResponse(ctx,
					"query depth %d exceeds maximum allowed depth of %d", depth, maxDepth))
			}
		}
		return next(ctx)
	})

	// ── Introspection control ──
	// Disable introspection in production to prevent schema leakage
	if cfg.Server.Mode == "release" {
		srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
			oc := graphql.GetOperationContext(ctx)
			if oc.Operation != nil && oc.Operation.Operation == ast.Query {
				// Block introspection queries (__schema, __type).
				for _, sel := range oc.Operation.SelectionSet {
					if field, ok := sel.(*ast.Field); ok {
						if field.Name == "__schema" || field.Name == "__type" {
							return graphql.OneShot(graphql.ErrorResponse(ctx, "introspection is disabled in production"))
						}
					}
				}
			}
			return next(ctx)
		})
	}

	// ── Prometheus Metrics ──
	srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		oc := graphql.GetOperationContext(ctx)
		opName := oc.OperationName
		if opName == "" {
			opName = "anonymous"
		}
		// Cap operation name length to prevent label cardinality explosion
		// from arbitrary client-supplied names
		if len(opName) > 64 {
			opName = "unknown"
		}

		timer := prometheus.NewTimer(graphqlDurationSeconds.WithLabelValues(opName))
		defer timer.ObserveDuration()

		resp := next(ctx)
		status := "success"
		if resp != nil && len(resp.Errors) > 0 {
			status = "error"
		}
		graphqlRequestsTotal.WithLabelValues(opName, status).Inc()
		return resp
	})

	// ── Error Masking ──
	if cfg.Server.Mode == "release" {
		srv.SetErrorPresenter(func(ctx context.Context, err error) *gqlerror.Error {
			// In production, don't leak internal error details
			gqlErr := graphql.DefaultErrorPresenter(ctx, err)
			msg := gqlErr.Message
			// Allow auth/access errors through (users need to know why access failed)
			switch msg {
			case "unauthorized: authentication required",
				"forbidden: admin access required",
				"invalid credentials",
				"invalid email or password",
				"account is disabled",
				"email already registered",
				"invalid or expired token",
				"invalid or expired reset token",
				"insufficient balance",
				"rate limit exceeded",
				"rate limit exceeded: try again later":
				return gqlErr
			}
			// Allow password validation errors through
			if strings.HasPrefix(msg, "password ") {
				return gqlErr
			}
			// Mask everything else (prevents info leakage for:
			// - plan existence probing ("plan not available")
			// - coupon code enumeration ("coupon expired or invalid")
			// - redeem code enumeration ("invalid redeem code", "redeem code already used")
			// - internal DB/service errors)
			gqlErr.Message = "internal error"
			return gqlErr
		})
	}

	return &Handler{server: srv, logger: logger}
}

// selectionDepth computes the maximum nesting depth of a selection set.
func selectionDepth(ss ast.SelectionSet) int {
	max := 0
	for _, sel := range ss {
		child := 0
		switch s := sel.(type) {
		case *ast.Field:
			if len(s.SelectionSet) > 0 {
				child = selectionDepth(s.SelectionSet)
			}
		case *ast.InlineFragment:
			if len(s.SelectionSet) > 0 {
				child = selectionDepth(s.SelectionSet)
			}
		case *ast.FragmentSpread:
			// Fragment spreads are resolved later; count as 1 level.
			child = 1
		}
		d := 1 + child
		if d > max {
			max = d
		}
	}
	return max
}

// ServeGraphQL returns a Gin handler for the GraphQL endpoint.
func (h *Handler) ServeGraphQL() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Inject Gin context into the Go context so directives/resolvers can access it
		ctx := context.WithValue(c.Request.Context(), directives.GinContextKey, c)
		c.Request = c.Request.WithContext(ctx)
		h.server.ServeHTTP(c.Writer, c.Request)
	}
}

// ServePlayground returns a Gin handler for the GraphQL Playground.
func ServePlayground() gin.HandlerFunc {
	h := playground.Handler("LLM Router GraphQL", "/graphql")
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
