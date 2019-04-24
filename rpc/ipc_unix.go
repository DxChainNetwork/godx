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

// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

/*
sockaddr_un structure:

struct sockaddr_un {
	sa_family_t sun_family
	char sun_path[108];
}
*/

package rpc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/DxChainNetwork/godx/log"
)

/*
#include <sys/un.h>

int max_socket_path_size() {
struct sockaddr_un s;
return sizeof(s.sun_path);
}
*/
import "C"

// ipcListen will create a Unix socket on the given endpoint.
//
// Unix socket identify server using the pathname. The client and server have to agree on the pathname
// for them to find each other.
func ipcListen(endpoint string) (net.Listener, error) {
	// the path name cannot be greater than the array size defined in sockaddr_un structure
	// which is 108 characters
	if len(endpoint) > int(C.max_socket_path_size()) {
		log.Warn(fmt.Sprintf("The ipc endpoint is longer than %d characters. ", C.max_socket_path_size()),
			"endpoint", endpoint)
	}

	// Ensure the IPC path exists and remove any previous leftover
	//
	// if the directory already existed, do nothing
	// otherwise, create the directory with permission code 0751
	// user -> read/write/execute  group -> read/execute other -> execute
	//
	// permission can be used as authentication
	if err := os.MkdirAll(filepath.Dir(endpoint), 0751); err != nil {
		return nil, err
	}
	// if endpoint already existed, remove it
	os.Remove(endpoint)

	// listen to the endpoint, which is a file, over unix network
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, err
	}
	// give user permission to read/write the file
	os.Chmod(endpoint, 0600)
	return l, nil
}

// newIPCConnection will connect to a Unix socket on the given endpoint.
// local file system
func newIPCConnection(ctx context.Context, endpoint string) (net.Conn, error) {
	return dialContext(ctx, "unix", endpoint)
}
