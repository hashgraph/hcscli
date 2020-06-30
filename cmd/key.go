package cmd

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go"
	"github.com/spf13/cobra"
)

func buildKeyCommand() *cobra.Command {
	genKeyCmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate ed25519 private key and public key pair",
		RunE:  genKey,
	}

	getPubKeyCmd := &cobra.Command{
		Use:   "pub [privKey]",
		Short: "Get ed25519 public key from the private key",
		Args:  cobra.ExactArgs(1),
		RunE:  getPubKey,
	}

	keyCmd := &cobra.Command{
		Use:   "key",
		Short: "Generate ed25519 public key and private key",
	}

	keyCmd.AddCommand(genKeyCmd)
	keyCmd.AddCommand(getPubKeyCmd)

	return keyCmd
}

func genKey(cmd *cobra.Command, args []string) error {
	privateKey, err := hedera.GenerateEd25519PrivateKey()
	if err != nil {
		return err
	}

	fmt.Printf("ed25519 private key - %s\ned25519 public key - %s\n", privateKey, privateKey.PublicKey())
	return nil
}

func getPubKey(cmd *cobra.Command, args []string) error {
	privateKey, err := hedera.Ed25519PrivateKeyFromString(args[0])
	if err != nil {
		return err
	}

	fmt.Printf("ed25519 public key - %s\n", privateKey.PublicKey())
	return nil
}
