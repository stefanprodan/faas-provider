// Copyright 2019 OpenFaaS Authors
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package v1

import (
	"fmt"
	"github.com/openfaas/faas-provider/auth/v1"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	types "github.com/openfaas/faas-provider/types/v1"
)

var r *mux.Router

// Mark this as a Golang "package"
func init() {
	r = mux.NewRouter()
}

// Router gives access to the underlying router for when new routes need to be added.
func Router() *mux.Router {
	return r
}

// Serve load your handlers into the correct OpenFaaS route spec. This function is blocking.
func Serve(handlers *types.FaaSHandlers, config *types.FaaSConfig) {

	if config.EnableBasicAuth {
		reader := v1.ReadBasicAuthFromDisk{
			SecretMountPath: config.SecretMountPath,
		}

		credentials, err := reader.Read()
		if err != nil {
			log.Fatal(err)
		}

		handlers.FunctionReader = v1.DecorateWithBasicAuth(handlers.FunctionReader, credentials)
		handlers.DeployHandler = v1.DecorateWithBasicAuth(handlers.DeployHandler, credentials)
		handlers.DeleteHandler = v1.DecorateWithBasicAuth(handlers.DeleteHandler, credentials)
		handlers.UpdateHandler = v1.DecorateWithBasicAuth(handlers.UpdateHandler, credentials)
		handlers.ReplicaReader = v1.DecorateWithBasicAuth(handlers.ReplicaReader, credentials)
		handlers.ReplicaUpdater = v1.DecorateWithBasicAuth(handlers.ReplicaUpdater, credentials)
		handlers.InfoHandler = v1.DecorateWithBasicAuth(handlers.InfoHandler, credentials)
		handlers.SecretHandler = v1.DecorateWithBasicAuth(handlers.SecretHandler, credentials)
	}

	// System (auth) endpoints
	r.HandleFunc("/system/functions", handlers.FunctionReader).Methods("GET")
	r.HandleFunc("/system/functions", handlers.DeployHandler).Methods("POST")
	r.HandleFunc("/system/functions", handlers.DeleteHandler).Methods("DELETE")
	r.HandleFunc("/system/functions", handlers.UpdateHandler).Methods("PUT")

	r.HandleFunc("/system/function/{name:[-a-zA-Z_0-9]+}", handlers.ReplicaReader).Methods("GET")
	r.HandleFunc("/system/scale-function/{name:[-a-zA-Z_0-9]+}", handlers.ReplicaUpdater).Methods("POST")
	r.HandleFunc("/system/info", handlers.InfoHandler).Methods("GET")

	r.HandleFunc("/system/secrets", handlers.SecretHandler).Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)

	// Open endpoints
	r.HandleFunc("/function/{name:[-a-zA-Z_0-9]+}", handlers.FunctionProxy)
	r.HandleFunc("/function/{name:[-a-zA-Z_0-9]+}/", handlers.FunctionProxy)
	r.HandleFunc("/function/{name:[-a-zA-Z_0-9]+}/{params:.*}", handlers.FunctionProxy)

	if config.EnableHealth {
		r.HandleFunc("/healthz", handlers.Health).Methods("GET")
	}

	readTimeout := config.ReadTimeout
	writeTimeout := config.WriteTimeout

	tcpPort := 8080
	if config.TCPPort != nil {
		tcpPort = *config.TCPPort
	}

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", tcpPort),
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes, // 1MB - can be overridden by setting Server.MaxHeaderBytes.
		Handler:        r,
	}

	log.Fatal(s.ListenAndServe())
}
