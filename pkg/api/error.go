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
	"net/url"

	"github.com/stoewer/go-strcase"
)

type (
	// RedirectError is used to redirect it the url is valid
	RedirectError struct {
		URL            *url.URL
		Status         int
		ErrDescription string
	}
)

var (
	// statusErrorMap maps http status to an error code string
	statusErrorMap = map[int]string{
		http.StatusBadRequest:          "bad_request",
		http.StatusUnauthorized:        "access_denied",
		http.StatusForbidden:           "forbidden",
		http.StatusNotFound:            "not_found",
		http.StatusConflict:            "conflict",
		http.StatusInternalServerError: "server_error",
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

// ErrorRedirect does a redirect if there u is valid
func ErrorRedirect(u *url.URL, status int, f string, args ...interface{}) *Response {
	if status == 0 {
		status = http.StatusInternalServerError
	}

	msg := fmt.Sprintf(f, args...)

	if u != nil {
		errCode := strcase.SnakeCase(http.StatusText(status))

		return Redirect(u, map[string]string{
			"error":             errCode,
			"error_description": msg,
		})
	}

	return StatusErrorf(status, msg)
}

// StatusError sets the status and error message in one go
func StatusError(status int, e error) *Response {
	return Error(e).WithStatus(status)
}

// StatusErrorf sets the status and error message in one go
func StatusErrorf(status int, f string, args ...interface{}) *Response {
	return Errorf(f, args...).WithStatus(status)
}
