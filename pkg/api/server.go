/*
 * Copyright (C) 2020 Atomic Media Foundation
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file in the root of this
 * workspace for details.
 */

// Package api is the atomic api helper library
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"reflect"
	"sync"

	"github.com/apex/log"
	"github.com/go-openapi/runtime"
	"github.com/gorilla/mux"
)

type (
	// Authorizer performs an autorization and returns a context or error on failure
	Authorizer func(r *http.Request) (interface{}, error)

	// BasicContext is provided by the basic authorizer
	BasicContext interface {
		Username() string
		Permissions() []string
	}

	basicContext struct {
		username    string
		permissions []string
	}

	// Server is an http server that provides basic REST funtionality
	Server struct {
		log           log.Interface
		router        *mux.Router
		apiRouter     *mux.Router
		addr          string
		listener      net.Listener
		srv           *http.Server
		lock          sync.Mutex
		basePath      string
		name          string
		version       string
		serverVersion string
		versioning    bool
	}

	// Parameters interface handles binding requests
	Parameters interface {
		BindRequestW(w http.ResponseWriter, r *http.Request, c ...runtime.Consumer) error
	}

	// Option provides the server options, these will override th defaults and any atomic
	// instance values.
	Option func(*Server)
)

// NewServer creates a new server object
func NewServer(opts ...Option) *Server {
	const (
		defaultAddr     = "127.0.0.1:9000"
		defaultBasePath = "/api/{version}"
		defaultName     = "Atomic"
		defaultVersion  = "1.0.0"
	)

	s := &Server{
		log:        log.Log,
		router:     mux.NewRouter(),
		addr:       defaultAddr,
		name:       defaultName,
		version:    defaultVersion,
		versioning: false,
		basePath:   defaultBasePath,
	}

	for _, opt := range opts {
		opt(s)
	}

	s.apiRouter = s.router.PathPrefix(s.basePath).Subrouter()

	s.apiRouter.Use(s.logMiddleware())

	if s.versioning {
		s.apiRouter.Use(s.versionMiddleware())
	}

	return s
}

// Serve starts the http server
func (s *Server) Serve() error {
	var listener net.Listener
	var err error

	s.lock.Lock()
	defer s.lock.Unlock()

	if s.srv != nil {
		return errors.New("server already running")
	}

	s.srv = &http.Server{
		Handler: s.router,
	}

	if s.listener != nil {
		listener = s.listener
	} else if s.addr != "" {
		listener, err = net.Listen("tcp", s.addr)
		if err != nil {
			return err
		}
	} else {
		return errors.New("server address not set")
	}

	go func() {
		if err := s.srv.Serve(listener); err != nil && err != http.ErrServerClosed {
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
func (s *Server) AddRoute(path string, method string, params Parameters, handler interface{}, auth ...Authorizer) {
	s.apiRouter.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		var err error
		var ctx interface{}
		var resp interface{}

		if len(auth) > 0 && auth[0] != nil {
			ctx, err = auth[0](r)
			if err != nil {
				s.log.Error(err.Error())
				s.WriteError(w, http.StatusUnauthorized, err)
				return
			}
		}

		defer func() {
			switch r := resp.(type) {
			case Responder:
				if err := r.Write(w); err != nil {
					s.log.Error(err.Error())
					s.WriteError(w, http.StatusInternalServerError, err)
				}
			case error:
				s.WriteError(w, http.StatusInternalServerError, err)
			}
		}()

		if h, ok := handler.(func(http.ResponseWriter, *http.Request, interface{}) Responder); ok {
			resp = h(w, r, ctx)
			return
		}

		var pv, cv reflect.Value

		if ctx != nil {
			cv = reflect.ValueOf(ctx)
		}

		if params != nil {
			pt := reflect.TypeOf(params)
			if pt.Kind() == reflect.Ptr {
				pt = pt.Elem()
			}
			params = reflect.New(pt).Interface().(Parameters)

			if err := params.BindRequestW(w, r); err != nil {
				s.log.Error(err.Error())
				s.WriteError(w, http.StatusBadRequest, err)
				return
			}

			pv = reflect.ValueOf(params)
		} else {
			pv = reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem())
		}

		fn := reflect.ValueOf(handler)
		args := []reflect.Value{}

		if fn.Type().NumIn() > 0 {
			args = append(args, pv)
		}
		if fn.Type().NumIn() == 2 {
			if !cv.IsValid() {
				cv = reflect.Zero(fn.Type().In(1))
			}

			args = append(args, cv)
		}

		rval := fn.Call(args)

		if len(rval) > 0 {
			resp = rval[0].Interface()
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
		s.log.Error(err.Error())
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

// Log specifies a new logger
func Log(l log.Interface) Option {
	return func(s *Server) {
		if l != nil {
			s.log = l
		}
	}
}

// Router specifies the router to use
func Router(router *mux.Router) Option {
	return func(s *Server) {
		if router != nil {
			s.router = router
			s.apiRouter = s.router.PathPrefix(s.basePath).Subrouter()

			if s.versioning {
				s.apiRouter.Use(s.versionMiddleware())
			}
		}
	}
}

// Addr sets the listen address for the server
func Addr(addr string) Option {
	return func(s *Server) {
		if addr != "" {
			s.addr = addr
		}
	}
}

// Listener sets the net listener for the server
func Listener(l net.Listener) Option {
	return func(s *Server) {
		s.listener = l
	}
}

// Basepath sets the router basepath for the api
func Basepath(base string) Option {
	return func(s *Server) {
		s.basePath = base
	}
}

// Versioning enables versioning that will enforce a versioned path
// and optionally set the Server header to the serverVersion
func Versioning(version string, serverVersion ...string) Option {
	return func(s *Server) {
		s.versioning = true
		s.version = version

		if len(serverVersion) > 0 {
			s.serverVersion = serverVersion[0]
		} else {
			s.serverVersion = version
		}
	}
}

// Name specifies the server name
func Name(name string) Option {
	return func(s *Server) {
		s.name = name
	}
}

// Log returns the server log
func (s *Server) Log() log.Interface {
	return s.log
}

// BasicAuthorizer implements a simple http basic authorizer that takes a callback for validating the user and password
func BasicAuthorizer(handler func(user string, pass string) ([]string, error)) Authorizer {
	return func(r *http.Request) (interface{}, error) {
		u, p, ok := r.BasicAuth()
		if !ok {
			return nil, errors.New("invalid authorization header")
		}

		ctx := &basicContext{
			username: u,
		}

		perms, err := handler(u, p)
		if err != nil {
		}

		ctx.permissions = perms

		return ctx, nil
	}
}

func (c *basicContext) Username() string {
	return c.username
}

func (c *basicContext) Permissions() []string {
	return c.permissions
}
