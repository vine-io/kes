package builder

import (
	"github.com/vine-io/kes/apiserver-runtime/internal/sample-apiserver/pkg/cmd/server"
)

// DisableAdmissionControllers disables delegated authentication and authorization
func (a *Server) DisableAdmissionControllers() *Server {
	server.ServerOptionsFns = append(server.ServerOptionsFns, func(o *ServerOptions) *ServerOptions {
		o.RecommendedOptions.Admission = nil
		return o
	})
	return a
}
