package cmd

import (
	"fmt"

	"github.com/hashgraph/hedera-sdk-go"
	"github.com/spf13/cobra"
)

func buildAccountCommand() *cobra.Command {
	balanceCmd := &cobra.Command{
		Use:   "balance [accountID]",
		Short: "Get account balance, default to the account in the config file",
		Args:  cobra.MaximumNArgs(1),
		RunE:  getBalanceForAccount,
	}

	accountCmd := &cobra.Command{
		Use:   "account",
		Short: "Account related commands",
	}

	accountCmd.AddCommand(balanceCmd)

	return accountCmd
}

func getBalanceForAccount(cmd *cobra.Command, args []string) (err error) {
	var accountID hedera.AccountID

	if len(args) != 0 {
		if accountID, err = hedera.AccountIDFromString(args[0]); err != nil {
			logger.Errorf("invalid account ID: %s", err)
			return err
		}
	} else {
		accountID = config.operatorID
	}

	client := createClient()

	balance, err := hedera.NewAccountBalanceQuery().SetAccountID(accountID).Execute(client)
	if err != nil {
		logger.Errorf("failed to run account balance query transaction: %s", err)
		return err
	}

	fmt.Printf("account: %s, balance: %s\n", accountID, balance)

	return nil
}
