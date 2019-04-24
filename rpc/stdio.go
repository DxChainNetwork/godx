// Copyright 2018 The go-ethereum Authors
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
	"errors"
	"net"
	"os"
	"time"
)

// DialStdIO creates a client on stdin/stdout.
//
// The connection function returns an empty stdioConn structure
func DialStdIO(ctx context.Context) (*Client, error) {
	// _ context.Context means ignores the parameter context.Context
	// the parameter is used to make sure the return function can be assigned to Client connectFunc
	stdioConnFunc := func(_ context.Context) (net.Conn, error) {
		return stdioConn{}, nil
	}

	return newClient(ctx, stdioConnFunc)
}

type stdioConn struct{}

// reade user's input from console, stores in b, returns number of bytes read
func (io stdioConn) Read(b []byte) (n int, err error) {
	return os.Stdin.Read(b)
}

// output b to the console, return number of bytes outputted
func (io stdioConn) Write(b []byte) (n int, err error) {
	return os.Stdout.Write(b)
}

// close the connection, which simply returns nil
func (io stdioConn) Close() error {
	return nil
}

// returns structure UnixAddr address
func (io stdioConn) LocalAddr() net.Addr {
	return &net.UnixAddr{Name: "stdio", Net: "stdio"}
}

// returns structure UnixAddr address
func (io stdioConn) RemoteAddr() net.Addr {
	return &net.UnixAddr{Name: "stdio", Net: "stdio"}
}

// set read and write deadlines
// for stdio, this feature is not supported
func (io stdioConn) SetDeadline(t time.Time) error {
	return &net.OpError{Op: "set", Net: "stdio", Source: nil, Addr: nil, Err: errors.New("deadline not supported")}
}

// set read deadlines
// for stdio, this feature is not supported
func (io stdioConn) SetReadDeadline(t time.Time) error {
	return &net.OpError{Op: "set", Net: "stdio", Source: nil, Addr: nil, Err: errors.New("deadline not supported")}
}

// set write deadlines
// for stdio, this feature is not supported
func (io stdioConn) SetWriteDeadline(t time.Time) error {
	return &net.OpError{Op: "set", Net: "stdio", Source: nil, Addr: nil, Err: errors.New("deadline not supported")}
}
