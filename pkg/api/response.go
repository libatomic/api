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
	WriterFunc func(w http.ResponseWriter, status int, payload interface{}, headers ...http.Header) error

	// Response is the common response type
	Response struct {
		status  int
		payload interface{}
		writer  WriterFunc
		header  http.Header
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
		header:  make(http.Header),
	}
}

// WithStatus sets the status
func (r *Response) WithStatus(status int) *Response {
	r.status = status
	return r
}

// WithHeader adds headers to the request
func (r *Response) WithHeader(key string, value string) *Response {
	r.header.Add(key, value)
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
func WriteJSON(w http.ResponseWriter, status int, payload interface{}, headers ...http.Header) error {
	if len(headers) > 0 {
		for key, vals := range headers[0] {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	return enc.Encode(payload)
}

// WriteXML writes an  object out as XML
func WriteXML(w http.ResponseWriter, status int, payload interface{}, headers ...http.Header) error {
	if len(headers) > 0 {
		for key, vals := range headers[0] {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
	}

	w.Header().Set("Content-Type", "application/xml")

	w.WriteHeader(status)

	enc := xml.NewEncoder(w)

	return enc.Encode(payload)
}

// Write writes the raw payload out expeting it to be bytes
func Write(w http.ResponseWriter, status int, payload interface{}, headers ...http.Header) error {
	if len(headers) > 0 {
		for key, vals := range headers[0] {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
	}

	w.WriteHeader(status)

	switch data := payload.(type) {
	case []byte:
		if _, err := w.Write(data); err != nil {
			return err
		}
	case string:
		if _, err := w.Write([]byte(data)); err != nil {
			return err
		}
	}

	return nil
}
