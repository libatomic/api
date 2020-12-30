/*
 * Copyright (C) 2020 Atomic Media Foundation
 *
 * This software may be modified and distributed under the terms
 * of the MIT license.  See the LICENSE file in the root of this
 * workspace for details.
 */

package api

import (
	"compress/gzip"
	"encoding/json"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cast"
)

type (
	// Responder is an api response interface
	Responder interface {
		// Status returns the http status
		Status() int

		// Payload is the payload
		Payload() interface{}

		// Write writes ou the payload
		Write(w http.ResponseWriter, r *http.Request) error
	}

	// EncodeFunc encodes the payload
	EncodeFunc func(w io.Writer) error

	// Response is the common response type
	Response struct {
		status  int
		payload interface{}
		header  http.Header
	}

	// Encoder is a response encoder
	Encoder interface {
		Encode(w io.Writer) error
	}
)

// NewResponse returns a response with defaults
func NewResponse(payload ...interface{}) *Response {
	var p interface{}
	if len(payload) > 0 {
		p = payload[0]
	}

	header := make(http.Header)
	header.Set("Content-Type", "application/json")

	return &Response{
		status:  http.StatusOK,
		payload: p,
		header:  header,
	}
}

// WithStatus sets the status
func (r *Response) WithStatus(status int) *Response {
	r.status = status
	return r
}

// WithHeader adds headers to the request
func (r *Response) WithHeader(key string, value string) *Response {
	r.header.Set(key, value)
	return r
}

// Redirect will set the proper redirect headers and http.StatusFound
func Redirect(u *url.URL, args ...map[string]string) *Response {
	r := NewResponse()

	q := u.Query()

	for _, a := range args {
		for k, v := range a {
			q.Set(k, v)
		}
	}

	u.RawQuery = q.Encode()

	r.header.Set("Location", u.String())

	r.status = http.StatusFound

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
func (r *Response) Write(w http.ResponseWriter, req *http.Request) error {
	if len(r.header) > 0 {
		for key, vals := range r.header {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
	}

	var out io.Writer
	out = w

	if strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
		wr := gzip.NewWriter(out)
		defer wr.Flush()
		out = wr
		w.Header().Set("Content-Encoding", "gzip")
	}

	if r.payload == nil {
		w.WriteHeader(r.status)
		return nil
	}

	w.WriteHeader(r.status)

	if closer, ok := r.payload.(io.Closer); ok {
		defer closer.Close()
	}

	switch t := r.payload.(type) {
	case []byte:
		if _, err := out.Write(t); err != nil {
			return err
		}

	case string:
		if _, err := out.Write([]byte(t)); err != nil {
			return err
		}

	case Encoder:
		return t.Encode(out)

	case io.Reader:
		if l := w.Header().Get("Content-Length"); l != "" {
			if _, err := io.CopyN(out, t, cast.ToInt64(l)); err != nil {
				return err
			}
		} else {
			if _, err := io.Copy(out, t); err != nil {
				return err
			}
		}

	default:
		switch r.header.Get("Content-Type") {
		case "application/xml":
			enc := xml.NewEncoder(out)
			return enc.Encode(r.payload)

		case "application/json":
			fallthrough
		default:
			enc := json.NewEncoder(out)
			return enc.Encode(r.payload)
		}
	}

	return nil
}
