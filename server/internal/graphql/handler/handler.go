// Package gqlhandler provides the GraphQL HTTP handler for the Gin router.
package gqlhandler

import (
	"context"
	"fmt"
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
	"llm-router-platform/pkg/sanitize"
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

	graphqlDepthRejected = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "depth_rejected_total",
			Help:      "Number of queries rejected due to depth limits.",
		},
	)

	graphqlAliasRejected = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "alias_rejected_total",
			Help:      "Number of queries rejected due to alias count limits.",
		},
	)

	graphqlErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "graphql",
			Name:      "errors_total",
			Help:      "GraphQL errors by type (client vs internal).",
		},
		[]string{"type"},
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
	srv.Use(extension.FixedComplexityLimit(200))

	// ── Introspection ──
	if cfg.FeatureGates.GraphQLIntrospection {
		srv.Use(extension.Introspection{})
	}

	setupQueryDepthLimiting(srv)
	setupAliasLimiting(srv)

	if !cfg.FeatureGates.GraphQLIntrospection {
		setupIntrospectionControl(srv)
	}

	setupPrometheusMetrics(srv)

	if cfg.Server.Mode == "release" {
		setupErrorMasking(srv, logger)
	}

	return &Handler{server: srv, logger: logger}
}

func setupQueryDepthLimiting(srv *handler.Server) {
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
}

func setupAliasLimiting(srv *handler.Server) {
	const maxAliases = 20
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		if oc.Operation != nil {
			count := countAliases(oc.Operation.SelectionSet)
			if count > maxAliases {
				graphqlAliasRejected.Inc()
				return graphql.OneShot(graphql.ErrorResponse(ctx,
					"query contains %d aliases, maximum allowed is %d", count, maxAliases))
			}
		}
		return next(ctx)
	})
}

func setupIntrospectionControl(srv *handler.Server) {
	srv.AroundOperations(func(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
		oc := graphql.GetOperationContext(ctx)
		if oc.Operation != nil && oc.Operation.Operation == ast.Query {
			for _, sel := range oc.Operation.SelectionSet {
				if field, ok := sel.(*ast.Field); ok {
					if field.Name == "__schema" || field.Name == "__type" {
						return graphql.OneShot(graphql.ErrorResponse(ctx, "introspection is disabled"))
					}
				}
			}
		}
		return next(ctx)
	})
}

func setupPrometheusMetrics(srv *handler.Server) {
	srv.AroundResponses(func(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
		oc := graphql.GetOperationContext(ctx)
		opName := oc.OperationName
		if opName == "" {
			opName = "anonymous"
		}
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
}

// Stable error codes surfaced in gqlErr.Extensions["code"]. Frontend clients
// branch on these instead of substring-matching the message text — see
// FE-H1 in the review document and the matching authLink in web/.
const (
	codeUnauthenticated     = "UNAUTHENTICATED"
	codeForbidden           = "FORBIDDEN"
	codeRateLimited         = "RATE_LIMITED"
	codeInvalidCredentials  = "INVALID_CREDENTIALS"
	codeNotFound            = "NOT_FOUND"
	codeInsufficientBalance = "INSUFFICIENT_BALANCE"
	codeAccountDisabled     = "ACCOUNT_DISABLED"
	codeCaptchaRequired     = "CAPTCHA_REQUIRED"
	codeValidation          = "VALIDATION"
	codeInternal            = "INTERNAL"
)

// classifyClientError returns a stable extension code for a known client-visible
// error message, or "" if the message is internal/unrecognized. The mapping is
// the source of truth for what the frontend can rely on without parsing text.
func classifyClientError(msg string) string {
	switch msg {
	case "unauthorized: authentication required",
		"invalid or expired token",
		"invalid refresh token",
		"refresh token has been revoked",
		"token has been revoked",
		"context canceled":
		return codeUnauthenticated
	case "forbidden: admin access required",
		"forbidden: IP not inside admin whitelist",
		"forbidden: access denied":
		return codeForbidden
	case "rate limit exceeded",
		"rate limit exceeded: try again later",
		"too many failed login attempts, please try again later":
		return codeRateLimited
	case "invalid credentials",
		"invalid email or password",
		"account not found":
		return codeInvalidCredentials
	case "account is disabled":
		return codeAccountDisabled
	case "insufficient balance":
		return codeInsufficientBalance
	case "CAPTCHA verification required",
		"CAPTCHA verification failed",
		"CAPTCHA verification failed, please try again",
		"too many failed attempts; CAPTCHA required but not configured on the server":
		return codeCaptchaRequired
	case "invalid or expired reset token",
		"record not found",
		"no organization found for user":
		return codeNotFound
	case "email already registered",
		"payments are currently disabled":
		return codeValidation
	}
	if strings.HasPrefix(msg, "too many failed login attempts") {
		return codeRateLimited
	}
	if strings.HasPrefix(msg, "password ") {
		return codeValidation
	}
	if strings.HasPrefix(msg, "query depth") || strings.HasPrefix(msg, "query contains") {
		return codeValidation
	}
	if isClientGraphQLError(msg) {
		return codeValidation
	}
	return ""
}

func setupErrorMasking(srv *handler.Server, logger *zap.Logger) {
	srv.SetErrorPresenter(func(ctx context.Context, err error) *gqlerror.Error {
		gqlErr := graphql.DefaultErrorPresenter(ctx, err)
		msg := gqlErr.Message

		if code := classifyClientError(msg); code != "" {
			graphqlErrorsTotal.WithLabelValues("client").Inc()
			if gqlErr.Extensions == nil {
				gqlErr.Extensions = map[string]interface{}{}
			}
			gqlErr.Extensions["code"] = code
			return gqlErr
		}

		graphqlErrorsTotal.WithLabelValues("internal").Inc()
		requestID := ""
		if gc, gcErr := directives.GinContextFromContext(ctx); gcErr == nil {
			requestID = sanitize.SafeString(gc.GetHeader("X-Request-ID"))
		}
		safeMsg := sanitize.SafeString(msg)
		logger.Warn("graphql internal error masked",
			zap.String("original_error", safeMsg),
			zap.String("request_id", requestID),
		)
		gqlErr.Message = fmt.Sprintf("internal error [%s]", requestID)
		gqlErr.Extensions = map[string]interface{}{
			"code":       codeInternal,
			"request_id": requestID,
		}
		return gqlErr
	})
}

func isClientGraphQLError(msg string) bool {
	clientPrefixes := []string{
		"Cannot query field ",
		"Unknown argument ",
		"Variable ",
		"Expected type ",
		"invalid base URL:",
		"invalid org ID",
		"invalid project ID",
		"provider not found",
		"server not found",
	}
	for _, prefix := range clientPrefixes {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}

	clientFragments := []string{
		"cannot use string as Int",
		"already exists",
		"is required but not provided",
	}
	for _, fragment := range clientFragments {
		if strings.Contains(msg, fragment) {
			return true
		}
	}

	return false
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

// countAliases recursively counts the total number of aliased fields in a
// selection set. Used to prevent alias-based amplification DoS attacks.
func countAliases(ss ast.SelectionSet) int {
	count := 0
	for _, sel := range ss {
		switch s := sel.(type) {
		case *ast.Field:
			if s.Alias != "" && s.Alias != s.Name {
				count++
			}
			count += countAliases(s.SelectionSet)
		case *ast.InlineFragment:
			count += countAliases(s.SelectionSet)
		}
	}
	return count
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
