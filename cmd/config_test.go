package cmd

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
	defaultConfigFile        = "../hedera_env.json"
	goodEd25519PrivateKeyStr = "302e020100300506032b657004220420fa14fab3f05d105669a56ff08d392f7d0d36a0e7e1811a83731f1ed4b5c7838a"
	goodEd25519PublicKeyStr1 = "302a300506032b6570032100a4076363d999db1ed230e53f797a6bd5575ab687df88a933210b05a7bb670f7f"
	goodEd25519PublicKeyStr2 = "302a300506032b6570032100d7cca8814b380ec98d6257c871205e9a27de30cfcb4c10b5173b980afb7a711c"
	goodEd25519PublicKeyStr3 = "302a300506032b6570032100a66190f84e3d94a07d06800860c65bb9c56884db5c39fbc437bbf98e1d6e72f3"
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
	testConfig := &struct {
		OperatorId        string
		OperatorKey       string
		Network           map[string]string
		MirrorNodeAddress string
	}{}
	if err := json.Unmarshal(data, &testConfig); err != nil {
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
				assert.Equal(t, val, config.operatorID.String(), "expected correct operatorId")
			} else {
				assert.Equal(t, testConfig.OperatorId, config.operatorID.String(), "expected correct operatorId")
			}

			if val, ok := tt.envVars[operatorKeyKey]; ok {
				assert.Equal(t, val, config.operatorKey.String(), "expected correct operatorPrivateKey")
			} else {
				assert.Equal(t, testConfig.OperatorKey, config.operatorKey.String(), "expected correct operatorPrivateKey")
			}

			if val, ok := tt.envVars[mirrorNodeAddressKey]; ok {
				assert.Equal(t, val, config.mirrorNodeAddress, "expected correct mirrorNodeAddress")
			} else {
				assert.Equal(t, testConfig.MirrorNodeAddress, config.mirrorNodeAddress, "expected correct mirrorNodeAddress")
			}

			wantNetwork := make(map[string]hedera.AccountID)
			var networkMap map[string]string
			if val, ok := tt.envVars[networkKey]; ok {
				err := json.Unmarshal([]byte(val), &networkMap)
				assert.NoError(t, err, "expected json unmarshal return no error")
			} else {
				networkMap = testConfig.Network
			}
			for key, val := range networkMap {
				acctId, err := hedera.AccountIDFromString(val)
				assert.Nil(t, err, "expected proper node id string")
				wantNetwork[key] = acctId
			}
			assert.True(t, reflect.DeepEqual(wantNetwork, config.network), "expected correct network")
		})
	}
}
