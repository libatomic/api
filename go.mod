module github.com/libatomic/api

go 1.14

replace github.com/libatomic/oauth => ../oauth

require (
	github.com/go-openapi/runtime v0.19.20
	github.com/gorilla/mux v1.7.4
	github.com/libatomic/oauth v1.0.0-alpha.16
	github.com/sirupsen/logrus v1.6.0
)
