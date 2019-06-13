// Copyright 2019 OpenFaaS Authors
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

/*

OpenFaaS API server example:

srv := bootstrap.NewApiServer()

srv.Handlers = &bootstrap.ApiServerHandlers{
	FunctionProxy:  handlers.MakeProxy(functionNamespace, srv.Config.ReadTimeout),
	DeleteHandler:  handlers.MakeDeleteHandler(functionNamespace, clientset),
	DeployHandler:  handlers.MakeDeployHandler(functionNamespace, factory),
	FunctionReader: handlers.MakeFunctionReader(functionNamespace, clientset),
	ReplicaReader:  handlers.MakeReplicaReader(functionNamespace, clientset),
	ReplicaUpdater: handlers.MakeReplicaUpdater(functionNamespace, clientset),
	UpdateHandler:  handlers.MakeUpdateHandler(functionNamespace, factory),
	HealthHandler:  handlers.MakeHealthHandler(),
	InfoHandler:    handlers.MakeInfoHandler(version.BuildVersion(), version.GitCommit),
	SecretHandler:  handlers.MakeSecretHandler(functionNamespace, clientset),
}

log.Fatal(srv.ListenAndServe(nil))

*/

package v2

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openfaas/faas-provider/auth/v1"
)

// ApiServer represents the OpenFaaS API
type ApiServer struct {
	Config   ApiServerConfig
	Handlers *ApiServerHandlers
}

// NewApiServer return an OpenFaaS API server
func NewApiServer() *ApiServer {
	return &ApiServer{
		Config: ReadConfig(),
	}
}

// ApiServerHandlers provide handlers for OpenFaaS API
type ApiServerHandlers struct {
	FunctionReader http.HandlerFunc
	DeployHandler  http.HandlerFunc
	DeleteHandler  http.HandlerFunc
	ReplicaReader  http.HandlerFunc
	ReplicaUpdater http.HandlerFunc
	SecretHandler  http.HandlerFunc

	// FunctionProxy provides the function invocation proxy logic. Use proxy.NewHandlerFunc to
	// use the standard OpenFaaS proxy implementation or provide completely custom proxy logic.
	FunctionProxy http.HandlerFunc

	// Optional: Update an existing function
	UpdateHandler http.HandlerFunc
	Health        http.HandlerFunc
	InfoHandler   http.HandlerFunc
}

// ListenAndServe load your handlers into the correct OpenFaaS route spec and starts the API server.
// This function is blocking.
func (srv *ApiServer) ListenAndServe(r *mux.Router) error {
	if r == nil {
		r = mux.NewRouter()
	}

	if srv.Handlers == nil {
		return fmt.Errorf("API handlers are missing")
	}

	if srv.Config.EnableBasicAuth {
		reader := v1.ReadBasicAuthFromDisk{
			SecretMountPath: srv.Config.SecretMountPath,
		}

		credentials, err := reader.Read()
		if err != nil {
			log.Fatal(err)
		}

		srv.Handlers.FunctionReader = v1.DecorateWithBasicAuth(srv.Handlers.FunctionReader, credentials)
		srv.Handlers.DeployHandler = v1.DecorateWithBasicAuth(srv.Handlers.DeployHandler, credentials)
		srv.Handlers.DeleteHandler = v1.DecorateWithBasicAuth(srv.Handlers.DeleteHandler, credentials)
		srv.Handlers.UpdateHandler = v1.DecorateWithBasicAuth(srv.Handlers.UpdateHandler, credentials)
		srv.Handlers.ReplicaReader = v1.DecorateWithBasicAuth(srv.Handlers.ReplicaReader, credentials)
		srv.Handlers.ReplicaUpdater = v1.DecorateWithBasicAuth(srv.Handlers.ReplicaUpdater, credentials)
		srv.Handlers.InfoHandler = v1.DecorateWithBasicAuth(srv.Handlers.InfoHandler, credentials)
		srv.Handlers.SecretHandler = v1.DecorateWithBasicAuth(srv.Handlers.SecretHandler, credentials)
	}

	// System (auth) endpoints
	r.HandleFunc("/system/functions", srv.Handlers.FunctionReader).Methods("GET")
	r.HandleFunc("/system/functions", srv.Handlers.DeployHandler).Methods("POST")
	r.HandleFunc("/system/functions", srv.Handlers.DeleteHandler).Methods("DELETE")
	r.HandleFunc("/system/functions", srv.Handlers.UpdateHandler).Methods("PUT")

	r.HandleFunc("/system/function/{name:[-a-zA-Z_0-9]+}", srv.Handlers.ReplicaReader).Methods("GET")
	r.HandleFunc("/system/scale-function/{name:[-a-zA-Z_0-9]+}", srv.Handlers.ReplicaUpdater).Methods("POST")
	r.HandleFunc("/system/info", srv.Handlers.InfoHandler).Methods("GET")

	r.HandleFunc("/system/secrets", srv.Handlers.SecretHandler).Methods(http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete)

	// Open endpoints
	r.HandleFunc("/function/{name:[-a-zA-Z_0-9]+}", srv.Handlers.FunctionProxy)
	r.HandleFunc("/function/{name:[-a-zA-Z_0-9]+}/", srv.Handlers.FunctionProxy)
	r.HandleFunc("/function/{name:[-a-zA-Z_0-9]+}/{params:.*}", srv.Handlers.FunctionProxy)

	if srv.Config.EnableHealth {
		r.HandleFunc("/healthz", srv.Handlers.Health).Methods("GET")
	}

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", srv.Config.ServerPort),
		ReadTimeout:    srv.Config.ReadTimeout,
		WriteTimeout:   srv.Config.WriteTimeout,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes, // 1MB - can be overridden by setting Server.MaxHeaderBytes.
		Handler:        r,
	}

	return s.ListenAndServe()
}
