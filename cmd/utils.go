package cmd

import  "github.com/hashgraph/hedera-sdk-go"

func createClient() *hedera.Client {
	client := hedera.NewClient(config.network)
	return client.SetOperator(config.operatorID, config.operatorKey)
}
