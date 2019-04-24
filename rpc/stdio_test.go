/*
	Functions not tested:
		* Read
		* Write
		* Close
*/

package rpc

import (
	"errors"
	"net"
	"testing"
	"time"
)

var unixAddrTest = net.UnixAddr{
	Name: "stdio",
	Net:  "stdio",
}

var stdioConnTest = stdioConn{}
var deadlineError error = &net.OpError{Op: "set", Net: "stdio", Source: nil, Addr: nil, Err: errors.New("deadline not supported")}

/*
	Test two functions LocalAddr and RemoteAddr to check if the output are same. Moreover, to check
	if the network and address for both local and remote are stdio
*/
func TestLocalAndRemoteAddr(t *testing.T) {
	localAddr := stdioConnTest.LocalAddr()
	remoteAddr := stdioConnTest.RemoteAddr()
	if localAddr.Network() != remoteAddr.Network() {
		t.Errorf("Local network: %s should be same as remote network: %s",
			localAddr.Network(), remoteAddr.Network())
	} else if localAddr.String() != remoteAddr.String() {
		t.Errorf("Local address: %s should be same as remote address: %s",
			localAddr.String(), remoteAddr.String())
	}

	if localAddr.Network() != "stdio" {
		t.Errorf("Both local network and remote network got %s, expected stdio", localAddr.Network())
	}

	if localAddr.String() != "stdio" {
		t.Errorf("Both local address and remote address got %s, expected stdio", localAddr.String())
	}
}

/*
	Test functions SetDeadline, SetReadDeadline, and SetWriteDeadline to check if they all returned same error
	&net.OpError{Op: "set", Net: "stdio", Source: nil, Addr: nil, Err: errors.New("deadline not supported")}
*/
func TestThreeDeadlineSet(t *testing.T) {
	deadlineTime := time.Time{}
	deadlineErr := stdioConnTest.SetDeadline(deadlineTime)
	rdeadlineErr := stdioConnTest.SetReadDeadline(deadlineTime)
	wdeadlineErr := stdioConnTest.SetWriteDeadline(deadlineTime)

	if deadlineErr.Error() != deadlineError.Error() {
		t.Errorf("Wrong error message for SetDeadline Function: got %s, expected %s",
			deadlineErr, deadlineError)
	}
	if rdeadlineErr.Error() != deadlineError.Error() {
		t.Errorf("Wrong error message for SetReadDeadline Function: got %s, expected %s",
			rdeadlineErr, deadlineError)
	}
	if wdeadlineErr.Error() != deadlineError.Error() {
		t.Errorf("Wrote error message for SetWriteDeadeline Function: got %s, expected %s",
			wdeadlineErr, deadlineError)
	}
}
