/*
 * Copyright (C) 2020 Atomic Media Foundation
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file in the root of this
 * workspace for details.
 */

package api

import (
	"errors"
	"fmt"
	"net/http"
)

type (
	redirectError struct {
		*Response
	}
)

// Error returns an error responder
func Error(e error) *Response {
	var r Responder

	if errors.As(e, &r) {
		return NewResponse(r.Payload()).WithStatus(r.Status())
	}

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

// StatusError sets the status and error message in one go
func StatusError(status int, e error) *Response {
	return Error(e).WithStatus(status)
}

// StatusErrorf sets the status and error message in one go
func StatusErrorf(status int, f string, args ...interface{}) *Response {
	return Errorf(f, args...).WithStatus(status)
}

// RedirectError returns a redirect error
func RedirectError(r *Response) error {
	return &redirectError{r}
}

func (e *redirectError) Error() string {
	return ""
}
