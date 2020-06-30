package cmd

import (
	"fmt"
	"github.com/hashgraph/hedera-sdk-go"
	"github.com/spf13/viper"
)

const (
	OPERATOR_ID_KEY         = "operatorId"
	OPERATOR_KEY_KEY        = "operatorKey"
	NETWORK_KEY             = "network"
	MIRROR_NODE_ADDRESS_KEY = "mirrorNodeAddress"
)

type HederaConfig struct {
	operatorID        hedera.AccountID
	operatorKey       hedera.Ed25519PrivateKey
	network           map[string]hedera.AccountID
	mirrorNodeAddress string
}

var config HederaConfig

func loadConfig(configFile string) error {
	v := viper.NewWithOptions(viper.KeyDelimiter("::"))
	v.AutomaticEnv()
	v.SetConfigName(configFile)
	v.SetConfigType("json")
	v.AddConfigPath(".")
	v.AddConfigPath("/")

	var err error
	if err = v.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration = %v", err)
	}

	var operatorId hedera.AccountID
	if operatorId, err = hedera.AccountIDFromString(v.GetString(OPERATOR_ID_KEY)); err != nil {
		return fmt.Errorf("invalid operator ID: %s", err)
	}

	var operatorKey hedera.Ed25519PrivateKey
	if operatorKey, err = hedera.Ed25519PrivateKeyFromString(v.GetString(OPERATOR_KEY_KEY)); err != nil {
		return fmt.Errorf("invalid operator Key: %s", err)
	}

	mirrorNodeAddress := v.GetString(MIRROR_NODE_ADDRESS_KEY)
	if mirrorNodeAddress == "" {
		return fmt.Errorf("empty mirror node address")
	}

	network := make(map[string]hedera.AccountID)
	// workaround for the viper issue that json string map in env var will not be converted to map[string]string
	networkVal := v.Get("network")
	var networkMap map[string]string
	switch val := networkVal.(type) {
	case string:
		networkMap = v.GetStringMapString(NETWORK_KEY)
	case map[string]interface{}:
		networkMap = v.GetStringMapString(NETWORK_KEY)
	default:
		return fmt.Errorf("invalid type of network - %T", val)
	}
	if len(networkMap) == 0 {
		return fmt.Errorf("empty network")
	}
	for address, id := range networkMap {
		nodeId, err := hedera.AccountIDFromString(id)
		if err != nil {
			return fmt.Errorf("invalid node id: %s", err)
		}
		network[address] = nodeId
	}

	config = HederaConfig{
		operatorID:        operatorId,
		operatorKey:       operatorKey,
		network:           network,
		mirrorNodeAddress: mirrorNodeAddress,
	}

	return nil
}
