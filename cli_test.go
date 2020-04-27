package main

import (
	"encoding/json"
	"github.com/hashgraph/hedera-sdk-go"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
)

const (
	defaultConfigFile        = "hedera_env.json"
	goodEd25519PrivateKeyStr = "302e020100300506032b657004220420fa14fab3f05d105669a56ff08d392f7d0d36a0e7e1811a83731f1ed4b5c7838a"
	operatorIdKey            = "OPERATORID"
	operatorKeyKey           = "OPERATORKEY"
	nodeIdKey                = "NODEID"
	nodeAddressKey           = "NODEADDRESS"
	networkKey               = "NETWORK"
	mirrorNodeAddressKey     = "MIRRORNODEADDRESS"
)

func TestLoadConfig(t *testing.T) {
	data, err := ioutil.ReadFile(defaultConfigFile)
	if err != nil {
		t.Fatalf("failed to read defafult configuration file = %v", err)
	}
	config := &struct {
		OperatorId        string
		OperatorKey       string
		NodeId            string
		NodeAddress       string
		Network           map[string]string
		MirrorNodeAddress string
	}{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to unmarshal the default configuration = %v", err)
	}

	tests := []struct {
		name    string
		envVars map[string]string
		wantErr bool
	}{
		{
			name: "Proper",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
			},
			wantErr: false,
		},
		{
			name: "ProperWithNodeIdAddressEnvVars",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
				nodeIdKey:      "0.0.3",
				nodeAddressKey: "0.testnet.hedera.com:50211",
			},
			wantErr: false,
		},
		{
			name: "ProperWithNetworkEnvVar",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
				networkKey:     `{"0.testnet.hedera.com:50211": "0.0.3"}`,
			},
			wantErr: false,
		},
		{
			name: "InvalidOperatorId",
			envVars: map[string]string{
				operatorIdKey:  "0.0 7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
			},
			wantErr: true,
		},
		{
			name: "InvalidOperatorKey",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: "this is a bad key",
			},
			wantErr: true,
		},
		{
			name: "InvalidNodeId",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
				nodeIdKey:      "invalid node id",
				nodeAddressKey: "0.testnet.hedera.com:50211",
			},
			wantErr: true,
		},
		{
			name: "InvalidNodeIdInNetwork",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
				networkKey:     `{"0.testnet.hedera.com:50211": "invalid node id"}`,
			},
			wantErr: true,
		},
		{
			name: "EmptyNetwork",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
				networkKey:     "{}",
			},
			wantErr: true,
		},
		{
			name: "MalformatNetwork",
			envVars: map[string]string{
				operatorIdKey:  "0.0.7650",
				operatorKeyKey: goodEd25519PrivateKeyStr,
				networkKey:     "malformat network",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				os.Clearenv()
			}()
			for key, val := range tt.envVars {
				os.Setenv(key, val)
			}

			err := loadConfig(defaultConfigFile)
			if tt.wantErr {
				assert.NotNil(t, err, "expected loadConfig to return error")
				return
			} else {
				assert.Nil(t, err, "expected loadConfig to return no error")
			}

			// verify configuration
			if val, ok := tt.envVars[operatorIdKey]; ok {
				assert.Equal(t, val, operatorId.String(), "expected correct operatorId")
			} else {
				assert.Equal(t, config.OperatorId, operatorId.String(), "expected correct operatorId")
			}

			if val, ok := tt.envVars[operatorKeyKey]; ok {
				assert.Equal(t, val, operatorPrivateKey.String(), "expected correct operatorPrivateKey")
			} else {
				assert.Equal(t, config.OperatorKey, operatorPrivateKey.String(), "expected correct operatorPrivateKey")
			}

			if val, ok := tt.envVars[mirrorNodeAddressKey]; ok {
				assert.Equal(t, val, mirrorNodeAddress, "expected correct mirrorNodeAddress")
			} else {
				assert.Equal(t, config.MirrorNodeAddress, mirrorNodeAddress, "expected correct mirrorNodeAddress")
			}

			wantNetwork := make(map[string]hedera.AccountID)
			var nodeIdVal, nodeAddressVal string
			if val, ok := tt.envVars[nodeIdKey]; ok && val != "" {
				nodeIdVal = val
			} else {
				nodeIdVal = config.NodeId
			}
			if val, ok := tt.envVars[nodeAddressKey]; ok && val != "" {
				nodeAddressVal = val
			} else {
				nodeAddressVal = config.NodeAddress
			}
			if nodeIdVal != "" && nodeAddressVal != "" {
				acctId, err := hedera.AccountIDFromString(nodeIdVal)
				assert.Nil(t, err, "expected proper node id string")
				wantNetwork[nodeAddressVal] = acctId
			} else {
				var networkMap map[string]string
				if val, ok := tt.envVars[networkKey]; ok {
					err := json.Unmarshal([]byte(val), &networkMap)
					assert.NoError(t, err, "expected json unmarshal return no error")
				} else {
					networkMap = config.Network
				}
				for key, val := range networkMap {
					acctId, err := hedera.AccountIDFromString(val)
					assert.Nil(t, err, "expected proper node id string")
					wantNetwork[key] = acctId
				}
			}
			assert.True(t, reflect.DeepEqual(wantNetwork, network), "expected correct network")
		})
	}
}
