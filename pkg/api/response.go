/*
 * Copyright (C) 2020 Atomic Media Foundation
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file in the root of this
 * workspace for details.
 */

package api

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
)

type (
	// Responder is an api response interface
	Responder interface {
		// Status returns the http status
		Status() int

		// Payload is the payload
		Payload() interface{}

		// Write writes ou the payload
		Write(w http.ResponseWriter) error
	}

	// WriterFunc is a response writer
	WriterFunc func(w http.ResponseWriter, status int, payload interface{}) error

	// Response is the common response type
	Response struct {
		status  int
		payload interface{}
		writer  WriterFunc
	}
)

// NewResponse returns a response with defaults
func NewResponse(payload ...interface{}) *Response {
	var p interface{}
	if len(payload) > 0 {
		p = payload[0]
	}
	return &Response{
		status:  http.StatusOK,
		payload: p,
		writer:  WriteJSON,
	}
}

// WithStatus sets the status
func (r *Response) WithStatus(status int) *Response {
	r.status = status
	return r
}

// WithWriter sets the writer
func (r *Response) WithWriter(w WriterFunc) *Response {
	r.writer = w
	return r
}

// Status returns the status
func (r *Response) Status() int {
	return r.status
}

// Payload returns the payload
func (r *Response) Payload() interface{} {
	return r.payload
}

// Write writes the response to the writer
func (r *Response) Write(w http.ResponseWriter) error {
	if r.payload == nil {
		w.WriteHeader(r.status)
		return nil
	}
	return r.writer(w, r.status, r.payload)
}

// WriteJSON writes json objects
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	return enc.Encode(payload)
}

// WriteXML writes an  object out as XML
func WriteXML(w http.ResponseWriter, status int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)

	enc := xml.NewEncoder(w)

	return enc.Encode(payload)
}
