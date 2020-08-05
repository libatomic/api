module github.com/libatomic/api

go 1.14

replace github.com/libatomic/oauth => ../oauth

require (
	github.com/apex/log v1.8.0
	github.com/blang/semver/v4 v4.0.0
	github.com/go-openapi/runtime v0.19.20
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.7.4
	github.com/libatomic/oauth v1.0.0-alpha.27
	github.com/sirupsen/logrus v1.6.0
	github.com/urfave/negroni v1.0.0
)
