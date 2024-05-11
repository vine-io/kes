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
	restclient "k8s.io/client-go/rest"
)

// PostStartHookFunc is a function that is called after the server has started.
// It must properly handle cases like:
//  1. asynchronous start in multiple API server processes
//  2. conflicts between the different processes all trying to perform the same action
//  3. partially complete work (API server crashes while running your hook)
//  4. API server access **BEFORE** your hook has completed
//
// Think of it like a mini-controller that is super privileged and gets to run in-process
// If you use this feature, tag @deads2k on github who has promised to review code for anyone's PostStartHook
// until it becomes easier to use.
type PostStartHookFunc func(context PostStartHookContext) error

// PreShutdownHookFunc is a function that can be added to the shutdown logic.
type PreShutdownHookFunc func() error

// PostStartHookContext provides information about this API server to a PostStartHookFunc
type PostStartHookContext struct {
	// LoopbackClientConfig is a config for a privileged loopback connection to the API server
	LoopbackClientConfig *restclient.Config
	// StopCh is the channel that will be closed when the server stops
	StopCh <-chan struct{}
}

// PostStartHookProvider is an interface in addition to provide a post start hook for the api server
type PostStartHookProvider interface {
	PostStartHook() (string, PostStartHookFunc, error)
}

type postStartHookEntry struct {
	hook PostStartHookFunc
	// originatingStack holds the stack that registered postStartHooks. This allows us to show a more helpful message
	// for duplicate registration.
	originatingStack string

	// done will be closed when the postHook is finished
	done chan struct{}
}

type PostStartHookConfigEntry struct {
	hook PostStartHookFunc
	// originatingStack holds the stack that registered postStartHooks. This allows us to show a more helpful message
	// for duplicate registration.
	originatingStack string
}

type preShutdownHookEntry struct {
	hook PreShutdownHookFunc
}
