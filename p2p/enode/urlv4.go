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

package enode

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"

	"github.com/DxChainNetwork/godx/common/math"
	"github.com/DxChainNetwork/godx/crypto"
	"github.com/DxChainNetwork/godx/p2p/enr"
)

var incompleteNodeURL = regexp.MustCompile("(?i)^(?:enode://)?([0-9a-f]+)$")

// MustParseV4 parses a node URL. It panics if the URL is not valid.
func MustParseV4(rawurl string) *Node {
	n, err := ParseV4(rawurl)
	if err != nil {
		panic("invalid node URL: " + err.Error())
	}
	return n
}

// ParseV4 parses a node URL.
//
// There are two basic forms of node URLs:
//
//   - incomplete nodes, which only have the public key (node ID)
//   - complete nodes, which contain the public key and IP/Port information
//
// For incomplete nodes, the designator must look like one of these
//
//    enode://<hex node id>
//    <hex node id>
//
// For complete nodes, the node ID is encoded in the username portion
// of the URL, separated from the host by an @ sign. The hostname can
// only be given as an IP address, DNS domain names are not allowed.
// The port in the host name section is the TCP listening port. If the
// TCP and UDP (discovery) ports differ, the UDP port is specified as
// query parameter "discport".
//
// In the following example, the node URL describes
// a node with IP address 10.3.58.6, TCP listening port 30303
// and UDP discovery port 30301.
//
//    enode://<hex node id>@10.3.58.6:30303?discport=30301
// node after parsing will have empty signature
func ParseV4(rawurl string) (*Node, error) {
	if m := incompleteNodeURL.FindStringSubmatch(rawurl); m != nil {
		// id is the public key
		id, err := parsePubkey(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid node ID (%v)", err)
		}
		return NewV4(id, nil, 0, 0), nil
	}
	return parseComplete(rawurl)
}

// NewV4 creates a node from discovery v4 node information. The record
// contained in the node has a zero-length signature.
//
// create node record and stores information inside
// create Node based on the information retrieved from the URL
func NewV4(pubkey *ecdsa.PublicKey, ip net.IP, tcp, udp int) *Node {
	var r enr.Record

	// update node Record's ip address, tcp and udp port information
	// all the of those information were stored in Record.pair
	if ip != nil {
		r.Set(enr.IP(ip))
	}
	if udp != 0 {
		r.Set(enr.UDP(udp))
	}
	if tcp != 0 {
		r.Set(enr.TCP(tcp))
	}

	// set empty signature and public key for node record
	// node address is derived from public key
	// == Keccak256 (pubkey)
	signV4Compat(&r, pubkey)

	// create new node. Node address is derived from the public key
	// which is Keccak256 (pubkey)
	n, err := New(v4CompatID{}, &r)
	if err != nil {
		panic(err)
	}
	return n
}

// parse the complete node url, which means IP and port information
// are included
func parseComplete(rawurl string) (*Node, error) {
	var (
		id               *ecdsa.PublicKey
		ip               net.IP
		tcpPort, udpPort uint64
	)

	// parse rawurl
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	// check if the url start with enode:
	if u.Scheme != "enode" {
		return nil, errors.New("invalid URL scheme, want \"enode\"")
	}
	// Parse the Node ID from the user portion.
	if u.User == nil {
		return nil, errors.New("does not contain node ID")
	}
	if id, err = parsePubkey(u.User.String()); err != nil {
		return nil, fmt.Errorf("invalid node ID (%v)", err)
	}
	// Parse the IP address.
	host, port, err := net.SplitHostPort(u.Host)

	if err != nil {
		return nil, fmt.Errorf("invalid host: %v", err)
	}

	// check and convert Host to IP address format
	// in here, IP is equivalent to host, but host is type of string
	// and ip is type of IP
	if ip = net.ParseIP(host); ip == nil {
		return nil, errors.New("invalid IP address")
	}

	// Ensure the IP is 4 bytes long for IPv4 addresses.
	// convert IP to 4 byte, which is IPv4 address
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}

	// Parse the port numbers
	// convert string to uint16
	if tcpPort, err = strconv.ParseUint(port, 10, 16); err != nil {
		return nil, errors.New("invalid port")
	}
	udpPort = tcpPort

	// check if udpPort is specified, if so, update udpPort
	qv := u.Query()
	if qv.Get("discport") != "" {
		udpPort, err = strconv.ParseUint(qv.Get("discport"), 10, 16)
		if err != nil {
			return nil, errors.New("invalid discport in query")
		}
	}

	return NewV4(id, ip, int(tcpPort), int(udpPort)), nil
}

// parsePubkey parses a hex-encoded secp256k1 public key.
// parses hex-encoded public key into secp256k1 public key
func parsePubkey(in string) (*ecdsa.PublicKey, error) {
	b, err := hex.DecodeString(in)
	if err != nil {
		return nil, err
	} else if len(b) != 64 {
		return nil, fmt.Errorf("wrong length, want %d hex chars", 128)
	}
	b = append([]byte{0x4}, b...)
	return crypto.UnmarshalPubkey(b)
}

// based on the Node information, creates enode URL (v4)
func (n *Node) v4URL() string {
	var (
		scheme enr.ID
		nodeid string
		key    ecdsa.PublicKey
	)

	// get the node's public key and scheme from node record
	n.Load(&scheme)
	n.Load((*Secp256k1)(&key))

	// if v4 scheme and has public key, then the node id will be public key
	// where first byte was removed.
	// Otherwise, the node id will be scheme.nodeID
	switch {
	// if node's public key is not empty and it is v4 scheme
	case scheme == "v4" || key != ecdsa.PublicKey{}:
		// converts public key to byte slice
		nodeid = fmt.Sprintf("%x", crypto.FromECDSAPub(&key)[1:])
	default:
		nodeid = fmt.Sprintf("%s.%x", scheme, n.id[:])
	}

	u := url.URL{Scheme: "enode"}
	if n.Incomplete() {
		u.Host = nodeid
	} else {
		addr := net.TCPAddr{IP: n.IP(), Port: n.TCP()}
		u.User = url.User(nodeid)
		u.Host = addr.String()
		if n.UDP() != n.TCP() {
			u.RawQuery = "discport=" + strconv.Itoa(n.UDP())
		}
	}
	return u.String()
}

// PubkeyToIDV4 derives the v4 node address from the given public key.
// based on the public key, get the node address, which is also the node id
func PubkeyToIDV4(key *ecdsa.PublicKey) ID {
	e := make([]byte, 64)
	math.ReadBits(key.X, e[:len(e)/2])
	math.ReadBits(key.Y, e[len(e)/2:])
	return ID(crypto.Keccak256Hash(e))
}
