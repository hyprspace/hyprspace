package cli

import (
	"fmt"
	"log"
	"os"

	"github.com/DataDrake/cli-ng/v2/cmd"
	"github.com/multiformats/go-multibase"
)

type KeygenFlags struct {
	PrivateKeyFile string `long:"private-key-file" desc:"Path to private key file."`
	PublicKeyFile  string `long:"public-key-file" desc:"Path to public key file. If not specified, prints to stdout."`
}

// Keygen creates a new keypair and writes each key to a file.
var Keygen = cmd.Sub{
	Name:  "keygen",
	Short: "Generate a new keypair",
	Flags: &KeygenFlags{},
	Run:   KeygenRun,
}

func KeygenRun(r *cmd.Root, c *cmd.Sub) {
	flags := c.Flags.(*KeygenFlags)

	if flags.PrivateKeyFile == "" {
		log.Fatal("--private-key-file is required")
	}

	keyBytes, peerId, err := GenerateKeyPair()
	checkErr(err)

	encoded := multibase.MustNewEncoder(multibase.Base58BTC).Encode(keyBytes)

	err = os.WriteFile(flags.PrivateKeyFile, []byte(encoded), 0600)
	checkErr(err)

	fmt.Fprintf(os.Stderr, "Private key written to %s\n", flags.PrivateKeyFile)

	if flags.PublicKeyFile != "" {
		err = os.WriteFile(flags.PublicKeyFile, []byte(peerId.String()), 0644)
		checkErr(err)

		fmt.Fprintf(os.Stderr, "Public key written to %s\n", flags.PublicKeyFile)
	} else {
		fmt.Println(peerId.String())
	}
}
