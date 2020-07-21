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
)

// Error returns an error responder
func Error(e error) *Response {
	p := struct {
		Message string `json:"message"`
	}{
		Message: e.Error(),
	}

	return NewResponse(p).WithStatus(http.StatusInternalServerError)
}

// Errorf returns a new error response from a string
func Errorf(f string, args ...interface{}) *Response {
	p := struct {
		Message string `json:"message"`
	}{
		Message: fmt.Sprintf(f, args...),
	}

	return NewResponse(p).WithStatus(http.StatusInternalServerError)
}
