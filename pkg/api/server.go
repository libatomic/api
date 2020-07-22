/*
 * Copyright (C) 2020 Atomic Media Foundation
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file in the root of this
 * workspace for details.
 */

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	"github.com/go-openapi/runtime"
	"github.com/gorilla/mux"
	"github.com/libatomic/oauth/pkg/oauth"
	"github.com/sirupsen/logrus"
)

type (
	// Server is an http server that provides basic REST funtionality
	Server struct {
		auth       oauth.Authorizer
		log        *logrus.Logger
		router     *mux.Router
		apiRouter  *mux.Router
		addr       string
		srv        *http.Server
		lock       sync.Mutex
		basePath   string
		name       string
		version    string
		versioning bool
	}

	// Parameters interface handles binding requests
	Parameters interface {
		BindRequest(r *http.Request, c ...runtime.Consumer) error
	}

	// Handler is a simple route handler function
	Handler func(params interface{}, ctx oauth.Context) Responder

	// Option provides the server options, these will override th defaults and any atomic
	// instance values.
	Option func(*Server)
)

// NewServer creates a new server object
func NewServer(opts ...Option) *Server {
	const (
		defaultAddr     = "127.0.0.1:9000"
		defaultBasePath = "/api"
		defaultName     = "Atomic"
		defaultVersion  = "1.0.0"
	)

	s := &Server{
		log:        logrus.StandardLogger(),
		router:     mux.NewRouter(),
		addr:       defaultAddr,
		name:       defaultName,
		version:    defaultVersion,
		versioning: true,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.apiRouter = s.router.PathPrefix(defaultBasePath).Subrouter()

	if s.versioning {
		s.apiRouter.Use(s.versionMiddleware())
	}

	return s
}

// Serve starts the http server
func (s *Server) Serve() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.srv != nil {
		return errors.New("server already running")
	}

	s.srv = &http.Server{
		Addr:    s.addr,
		Handler: s.router,
	}

	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Fatalf("listen: %s\n", err)
		}
	}()

	s.log.Debugf("http server listening on: %s", s.addr)

	return nil
}

// Shutdown shuts down the http server with the context
func (s *Server) Shutdown(ctx context.Context) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.srv == nil {
		s.log.Fatal("server already shutdown")
	}

	err := s.srv.Shutdown(ctx)

	s.srv = nil

	return err
}

// Handler returns the server http handler
func (s *Server) Handler() http.Handler {
	return s.router
}

// Router returns the server router
func (s *Server) Router() *mux.Router {
	return s.router
}

// AddRoute adds a route in the clear
func (s *Server) AddRoute(path string, method string, params Parameters, h Handler, scope ...[]string) {
	s.apiRouter.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		var err error
		var ctx oauth.Context

		if s.auth != nil && len(scope) > 0 {
			ctx, err = s.auth.AuthorizeRequest(r, scope...)
			if err != nil {
				if err == oauth.ErrAccessDenied {
					s.WriteError(w, http.StatusUnauthorized, err)
					return
				}
				s.log.Errorln(err)
				s.WriteError(w, http.StatusBadRequest, err)
				return
			}
		}

		if err := params.BindRequest(r); err != nil {
			s.log.Errorln(err)
			s.WriteError(w, http.StatusBadRequest, err)
			return
		}

		resp := h(params, ctx)

		if err := resp.Write(w); err != nil {
			s.log.Errorln(err)
			s.WriteError(w, http.StatusInternalServerError, err)
		}
	}).Methods(method)
}

// WriteJSON writes out json
func (s *Server) WriteJSON(w http.ResponseWriter, status int, v interface{}, pretty ...bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	if len(pretty) > 0 && pretty[0] {
		enc.SetIndent("", "\t")
	}

	if err := enc.Encode(v); err != nil {
		s.log.Errorln(err)
	}
}

// WriteError writes an error object
func (s *Server) WriteError(w http.ResponseWriter, status int, err error) {
	out := struct {
		Message string `json:"message"`
	}{
		Message: err.Error(),
	}

	s.WriteJSON(w, status, out)
}

// WithLogger specifies a new logger
func WithLogger(logger *logrus.Logger) Option {
	return func(s *Server) {
		if logger != nil {
			s.log = logger
		}
	}
}

// WithRouter specifies the router to use
func WithRouter(router *mux.Router) Option {
	return func(s *Server) {
		if router != nil {
			s.router = router
		}
	}
}

// WithAddr sets the listen address for the server
func WithAddr(addr string) Option {
	return func(s *Server) {
		if addr != "" {
			s.addr = addr
		}
	}
}

// WithAuthorizer sets the authorizer for the server
func WithAuthorizer(auth oauth.Authorizer) Option {
	return func(s *Server) {
		s.auth = auth
	}
}

// WithBasepath sets the router basepath for the api
func WithBasepath(base string) Option {
	return func(s *Server) {
		s.basePath = base
	}
}

// WithVersioning enables or disables the versioning middleware
func WithVersioning(enabled bool, version ...string) Option {
	return func(s *Server) {
		s.versioning = enabled

		if enabled && len(version) > 0 {
			s.version = version[0]
		}
	}
}

// WithName specifies the server name
func WithName(name string) Option {
	return func(s *Server) {
		s.name = name
	}
}
