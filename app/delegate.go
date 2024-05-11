/*
   Copyright 2024 The kes Authors

   This program is offered under a commercial and under the AGPL license.
   For AGPL licensing, see below.

   AGPL licensing:
   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package app

import (
	"net/http"

	"k8s.io/apiserver/pkg/server/healthz"
)

// DelegationTarget is an interface which allows for composition of API servers with top level handling that works
// as expected.
type DelegationTarget interface {
	// UnprotectedHandler returns a handler that is NOT protected by a normal chain
	UnprotectedHandler() http.Handler

	// PostStartHooks returns the post-start hooks that need to be combined
	PostStartHooks() map[string]postStartHookEntry

	// PreShutdownHooks returns the pre-stop hooks that need to be combined
	PreShutdownHooks() map[string]preShutdownHookEntry

	// HealthzChecks returns the healthz checks that need to be combined
	HealthzChecks() []healthz.HealthChecker

	// ListedPaths returns the paths for supporting an index
	ListedPaths() []string

	// NextDelegate returns the next delegationTarget in the chain of delegations
	NextDelegate() DelegationTarget

	// PrepareRun does post API installation setup steps. It calls recursively the same function of the delegates.
	PrepareRun() preparedGenericAPIServer

	// MuxAndDiscoveryCompleteSignals exposes registered signals that indicate if all known HTTP paths have been installed.
	MuxAndDiscoveryCompleteSignals() map[string]<-chan struct{}

	// Destroy cleans up its resources on shutdown.
	// Destroy has to be implemented in thread-safe way and be prepared
	// for being called more than once.
	Destroy()
}

type emptyDelegate struct {
	// handler is called at the end of the delegation chain
	// when a request has been made against an unregistered HTTP path the individual servers will simply pass it through until it reaches the handler.
	handler http.Handler
}

func NewEmptyDelegate() DelegationTarget {
	return emptyDelegate{}
}

// NewEmptyDelegateWithCustomHandler allows for registering a custom handler usually for special handling of 404 requests
func NewEmptyDelegateWithCustomHandler(handler http.Handler) DelegationTarget {
	return emptyDelegate{handler}
}

func (s emptyDelegate) UnprotectedHandler() http.Handler {
	return s.handler
}
func (s emptyDelegate) PostStartHooks() map[string]postStartHookEntry {
	return map[string]postStartHookEntry{}
}
func (s emptyDelegate) PreShutdownHooks() map[string]preShutdownHookEntry {
	return map[string]preShutdownHookEntry{}
}
func (s emptyDelegate) HealthzChecks() []healthz.HealthChecker {
	return []healthz.HealthChecker{}
}
func (s emptyDelegate) ListedPaths() []string {
	return []string{}
}
func (s emptyDelegate) NextDelegate() DelegationTarget {
	return nil
}
func (s emptyDelegate) PrepareRun() preparedGenericAPIServer {
	return preparedGenericAPIServer{}
}
func (s emptyDelegate) MuxAndDiscoveryCompleteSignals() map[string]<-chan struct{} {
	return map[string]<-chan struct{}{}
}
func (s emptyDelegate) Destroy() {
}

// preparedGenericAPIServer is a private wrapper that enforces a call of PrepareRun() before Run can be invoked.
type preparedGenericAPIServer struct {
}
