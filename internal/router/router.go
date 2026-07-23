package router

import (
	"net/http"

	"github.com/AnupamSingh2004/iiitone-backend/internal/auth"
	"github.com/AnupamSingh2004/iiitone-backend/internal/courses"
	"github.com/AnupamSingh2004/iiitone-backend/internal/materials"
	"github.com/AnupamSingh2004/iiitone-backend/internal/metrics"
	"github.com/AnupamSingh2004/iiitone-backend/internal/moderation"
	"github.com/AnupamSingh2004/iiitone-backend/internal/search"
	"github.com/AnupamSingh2004/iiitone-backend/internal/storage"
	"github.com/AnupamSingh2004/iiitone-backend/internal/users"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Deps holds everything the router needs. In tests, only JWTSecret is set —
// handlers that need a DB pool are exercised in their own package's tests, not here;
// this router test suite only checks routing/auth-gating shape.
type Deps struct {
	JWTSecret    string
	Pool         *pgxpool.Pool
	Cache        *redis.Client
	Store        storage.Store
	AuthLogin    http.Handler
	AuthCallback http.Handler
	FrontendURL  string
}

func New(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	m := metrics.New()
	r.Use(m.Middleware)
	r.Get("/metrics", m.Handler().ServeHTTP)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	if deps.AuthLogin != nil {
		r.Get("/auth/google/login", deps.AuthLogin.ServeHTTP)
	}
	if deps.AuthCallback != nil {
		r.Get("/auth/google/callback", deps.AuthCallback.ServeHTTP)
	}

	r.Route("/api", func(api chi.Router) {
		api.Use(auth.RequireAuth(deps.JWTSecret))

		// Routes are registered unconditionally, even when deps.Pool is nil
		// (as in this package's own unauthenticated-request smoke tests):
		// chi only invokes a subrouter's Use() middleware chain for a
		// request that matches a registered route — an unmatched path falls
		// straight through to the NotFoundHandler without ever running
		// RequireAuth, which would turn the smoke test's expected 401 into
		// a 404 instead. Every repo/handler constructor below only wraps
		// its *pgxpool.Pool pointer without dereferencing it, so this is
		// safe to build with a nil Pool; RequireAuth rejects unauthenticated
		// requests long before a handler would ever touch it.
		userRepo := users.NewRepository(deps.Pool)
		userHandlers := users.NewHandlers(userRepo)
		api.Get("/me", userHandlers.Me)
		api.Patch("/me", userHandlers.UpdateMe)

		courseRepo := courses.NewRepository(deps.Pool)
		courseHandlers := courses.NewHandlers(courseRepo)
		api.Get("/courses", courseHandlers.List)

		matRepo := materials.NewRepository(deps.Pool)
		uploadHandler := materials.NewUploadHandler(matRepo, courseRepo, deps.Store)
		api.Post("/materials", uploadHandler.ServeHTTP)

		searchRepo := search.NewRepository(deps.Pool, deps.Cache)
		searchHandlers := search.NewHandlers(searchRepo)
		api.Get("/search", searchHandlers.Search)

		flagsRepo := moderation.NewFlagsRepository(deps.Pool)
		modHandlers := moderation.NewHandlers(matRepo, flagsRepo, deps.Store)
		api.Post("/flags", modHandlers.CreateFlag)

		api.Route("/admin", func(admin chi.Router) {
			admin.Use(auth.RequireAdmin)
			admin.Get("/materials/pending", modHandlers.ListPending)
			admin.Post("/materials/{materialID}/approve", modHandlers.Approve)
			admin.Post("/materials/{materialID}/reject", modHandlers.Reject)
			admin.Get("/flags", modHandlers.ListOpenFlags)
			admin.Post("/flags/{flagID}/resolve", modHandlers.ResolveFlag)
			admin.Post("/users/{userID}/ban", userHandlers.Ban)
			admin.Post("/users/{userID}/unban", userHandlers.Unban)
		})
	})

	return r
}
