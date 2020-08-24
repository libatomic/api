/*
 * Copyright (C) 2020 Atomic Media Foundation
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file in the root of this
 * workspace for details.
 */

package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
)

type key int

const verKey key = 0

func (s *Server) versionMiddleware() func(http.Handler) http.Handler {
	apiVer, _ := semver.ParseTolerant(s.version)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)

			ver, ok := vars["version"]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			pathVer, err := semver.ParseTolerant(ver)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if pathVer.GT(apiVer) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			r.URL.Path = strings.Replace(r.URL.Path, ver, pathVer.String(), 1)

			context.Set(r, verKey, ver)

			w.Header().Set("Server", fmt.Sprintf("%s/%s", s.name, s.serverVersion))

			next.ServeHTTP(w, r)
		})
	}
}

// RequestVersion returns the request version or the server version is not found
func (s *Server) RequestVersion(r *http.Request) string {
	ver := s.version

	if val, ok := context.GetOk(r, verKey); ok {
		ver = val.(string)
	}

	return ver
}
