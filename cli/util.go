package cli

import (
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/mattn/go-isatty"
)

func printListF(strings []string, handler func(string) string) {
	for _, s := range strings {
		fmt.Printf("    %s\n", handler(s))
	}
}

func isTTY() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

func GenerateKeyPair() ([]byte, peer.ID, error) {
	var zeroID peer.ID

	privKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	if err != nil {
		return nil, zeroID, err
	}

	keyBytes, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, zeroID, err
	}

	peerId, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, zeroID, err
	}

	return keyBytes, peerId, nil
}
