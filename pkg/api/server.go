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
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"runtime/debug"
	"sync"

	"github.com/apex/log"
	"github.com/apex/log/handlers/discard"
	"github.com/go-openapi/runtime"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

type (
	// Authorizer performs an autorization and returns a context or error on failure
	Authorizer func(r *http.Request) (context.Context, error)

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

	routeOption struct {
		method      string
		params      interface{}
		contextFunc ContextFunc
		authorizers []Authorizer
	}

	// RouteOption defines route options
	RouteOption func(*routeOption)

	// Parameters interface handles binding requests
	Parameters interface {
		BindRequest(w http.ResponseWriter, r *http.Request, c ...runtime.Consumer) error
	}

	// ContextFunc adds context to a request
	ContextFunc func(context.Context) context.Context

	// Option provides the server options, these will override th defaults and any atomic
	// instance values.
	Option func(*Server)

	contextKey string

	requestContext struct {
		r *http.Request
		w http.ResponseWriter
	}
)

var (
	contextKeyLogger = contextKey("logger")

	contextKeyRequest = contextKey("request")
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

	s.apiRouter.Use(s.LogMiddleware())

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

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// AddRoute adds a route in the clear
func (s *Server) AddRoute(path string, handler interface{}, opts ...RouteOption) {
	opt := &routeOption{
		method: http.MethodGet,
		params: make(map[string]interface{}),
	}

	for _, o := range opts {
		o(opt)
	}

	s.apiRouter.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		var resp interface{}

		if len(opt.authorizers) > 0 && opt.authorizers[0] != nil {
			for _, a := range opt.authorizers {
				ctx, err := a(r)
				if err != nil {
					s.log.Error(err.Error())
					s.WriteError(w, http.StatusUnauthorized, err)
					return
				}

				// add the auth context to the context
				if ctx != nil {
					r = r.WithContext(ctx)
				}
			}
		}

		defer func() {
			if err := recover(); err != nil {
				debug.PrintStack()
			}

			switch r := resp.(type) {
			case Responder:
				if err := r.Write(w); err != nil {
					s.log.Error(err.Error())
					s.WriteError(w, http.StatusInternalServerError, err)
				}
			case error:
				s.WriteError(w, http.StatusInternalServerError, r)
			}
		}()

		// add the request object to the context
		r = r.WithContext(context.WithValue(r.Context(), contextKeyRequest, &requestContext{r, w}))

		// add the log to the context
		r = r.WithContext(context.WithValue(r.Context(), contextKeyLogger, s.log))

		// Add any additional context from the caller
		if opt.contextFunc != nil {
			r = r.WithContext(opt.contextFunc(r.Context()))
		}

		if h, ok := handler.(func(http.ResponseWriter, *http.Request) Responder); ok {
			resp = h(w, r)
			return
		} else if h, ok := handler.(func(http.ResponseWriter, *http.Request)); ok {
			h(w, r)
			return
		}

		var pv reflect.Value

		if opt.params != nil {
			if d, ok := opt.params.(Parameters); ok {
				if err := d.BindRequest(w, r); err != nil {
					s.log.Error(err.Error())
					s.WriteError(w, http.StatusBadRequest, err)
					return
				}
			} else {
				decoder := schema.NewDecoder()
				decoder.SetAliasTag("json")
				decoder.IgnoreUnknownKeys(true)

				vars := mux.Vars(r)
				if len(vars) > 0 {
					vals := make(url.Values)
					for k, v := range vars {
						vals.Add(k, v)
					}
					if err := decoder.Decode(opt.params, vals); err != nil {
						s.log.Error(err.Error())
						s.WriteError(w, http.StatusBadRequest, err)
						return
					}
				}

				if len(r.URL.Query()) > 0 {
					if err := decoder.Decode(opt.params, r.URL.Query()); err != nil {
						s.log.Error(err.Error())
						s.WriteError(w, http.StatusBadRequest, err)
						return
					}
				}

				if r.Body != nil {
					if r.Header.Get("Content-type") == "application/json" {
						data, err := ioutil.ReadAll(r.Body)
						if err != nil {
							s.log.Error(err.Error())
							s.WriteError(w, http.StatusBadRequest, err)
							return
						}

						if err := json.Unmarshal(data, opt.params); err != nil {
							s.log.Error(err.Error())
							s.WriteError(w, http.StatusBadRequest, err)
							return
						}
					} else if r.Header.Get("Content-type") == "application/x-www-form-urlencoded" {
						if err := r.ParseForm(); err != nil {
							s.log.Error(err.Error())
							s.WriteError(w, http.StatusBadRequest, err)
							return
						}

						if err := decoder.Decode(opt.params, r.Form); err != nil {
							s.log.Error(err.Error())
							s.WriteError(w, http.StatusBadRequest, err)
							return
						}
					}
				}
			}

			pv = reflect.ValueOf(opt.params)
		} else {
			pv = reflect.Zero(reflect.TypeOf((*interface{})(nil)).Elem())
		}

		fn := reflect.ValueOf(handler)
		args := []reflect.Value{}

		// support optional context as first parameter
		narg := 0
		if fn.Type().In(0) == reflect.TypeOf((*context.Context)(nil)).Elem() {
			args = append(args, reflect.ValueOf(r.Context()))
			narg++
		}
		if fn.Type().NumIn() > narg {
			args = append(args, pv)
		}

		rval := fn.Call(args)

		if len(rval) > 0 {
			resp = rval[0].Interface()
		}

	}).Methods(opt.method)
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

// WithLog specifies a new logger
func WithLog(l log.Interface) Option {
	return func(s *Server) {
		if l != nil {
			s.log = l
		}
	}
}

// WithRouter specifies the router to use
func WithRouter(router *mux.Router) Option {
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

// WithAddr sets the listen address for the server
func WithAddr(addr string) Option {
	return func(s *Server) {
		if addr != "" {
			s.addr = addr
		}
	}
}

// WithListener sets the net listener for the server
func WithListener(l net.Listener) Option {
	return func(s *Server) {
		s.listener = l
	}
}

// WithBasepath sets the router basepath for the api
func WithBasepath(base string) Option {
	return func(s *Server) {
		s.basePath = base
	}
}

// WithVersioning enables versioning that will enforce a versioned path
// and optionally set the Server header to the serverVersion
func WithVersioning(version string, serverVersion ...string) Option {
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

// WithName specifies the server name
func WithName(name string) Option {
	return func(s *Server) {
		s.name = name
	}
}

// WithMethod sets the method for the route option
func WithMethod(m string) RouteOption {
	return func(r *routeOption) {
		r.method = m
	}
}

// WithParams sets the params for the route option
func WithParams(p interface{}) RouteOption {
	return func(r *routeOption) {
		pt := reflect.TypeOf(p)
		if pt.Kind() == reflect.Ptr {
			pt = pt.Elem()
		}
		r.params = reflect.New(pt).Interface()
	}
}

// WithContextFunc sets the context handler for the route option
func WithContextFunc(f ContextFunc) RouteOption {
	return func(r *routeOption) {
		r.contextFunc = f
	}
}

// WithAuthorizers sets the authorizers
func WithAuthorizers(a ...Authorizer) RouteOption {
	return func(r *routeOption) {
		r.authorizers = a
	}
}

// Log returns the logger
func Log(ctx context.Context) log.Interface {
	l := ctx.Value(contextKeyLogger)
	if l != nil {
		return l.(log.Interface)
	}

	logger := &log.Logger{
		Handler: discard.Default,
	}

	return logger
}

// Request gets the reqest and response objects from the context
func Request(ctx context.Context) (*http.Request, http.ResponseWriter) {
	l := ctx.Value(contextKeyRequest)
	if r, ok := l.(*requestContext); ok {
		return r.r, r.w
	}
	return nil, nil
}

// Log returns the server log
func (s *Server) Log() log.Interface {
	return s.log
}
