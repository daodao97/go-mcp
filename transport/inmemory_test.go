package transport

import (
	"testing"
)

func TestInmemoryTransport(t *testing.T) {
	serverTransport := NewInmemoryServerTransport()
	clientTransport := NewInmemoryClientTransport(serverTransport)

	testTransport(t, clientTransport, serverTransport)
}

func TestInmemoryTransportPair(t *testing.T) {
	serverTransport, clientTransport := NewInmemoryTransportPair(nil, nil)

	testTransport(t, clientTransport, serverTransport)
}
