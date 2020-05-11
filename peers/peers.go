package peers

import (
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// A Peer is a combination of an IP address and Port to download stuff from.
type Peer struct {
	IP net.IP
	Port uint16
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}

// Unmarshal parses a buffer into a slice of Peer structs
func Unmarshal(peersBin []byte) ([]Peer, error) {
	const peerSize = 6
	numPeers := len(peersBin) / peerSize
	if len(peersBin) % peerSize != 0 {
		err := fmt.Errorf("Received malformed peers")
		return nil, err
	}

	// Create a slice of Peers with size of numPeers
	// numPeers is created by taking the mod of the size of the
	// buffer input based on the size of each peer in bytes.
	peers := make([]Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset + 4])
		peers[i].Port = binary.BigEndian.Uint16([]byte(peersBin[offset + 4 : offset + 6]))
	}

	return peers, nil
}
