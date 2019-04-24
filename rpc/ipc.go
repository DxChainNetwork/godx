// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rpc

import (
	"context"
	"net"

	"github.com/DxChainNetwork/godx/log"
	"github.com/DxChainNetwork/godx/p2p/netutil"
)

// ServeListener accepts connections on l, serving JSON-RPC on them.
//
// getting connection from listener and call handler function to handle the connection
// For the listener defined within RPC, it can listen to unix and windows listeners
func (srv *Server) ServeListener(l net.Listener) error {
	for {
		// waits and return the next connection
		conn, err := l.Accept()

		// Check if the connection error is temporary, if so, continue
		if netutil.IsTemporaryError(err) {
			log.Warn("IPC accept error", "err", err)
			continue
		} else if err != nil {
			return err
		}
		log.Trace("IPC accepted connection")
		go srv.ServeCodec(NewJSONCodec(conn), OptionMethodInvocation|OptionSubscriptions)
	}
}

// DialIPC create a new IPC client that connects to the given endpoint. On Unix it assumes
// the endpoint is the full path to a unix socket, and Windows the endpoint is an
// identifier for a named pipe.
//
// The context is used for the initial connection establishment. It does not
// affect subsequent interactions with the client.
func DialIPC(ctx context.Context, endpoint string) (*Client, error) {
	ipcConnFunc := func(ctx context.Context) (net.Conn, error) {
		// create and return unix socket connection to the given endpoint
		return newIPCConnection(ctx, endpoint)
	}

	return newClient(ctx, ipcConnFunc)
}
