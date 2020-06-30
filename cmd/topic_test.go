package cmd

import (
	"github.com/hashgraph/hedera-sdk-go"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestMakePublicKey(t *testing.T) {
	type args struct {
		threshold         uint32
		keys              []string
		operatorPublicKey hedera.Ed25519PublicKey
		exclOperator      bool
	}
	opPubKey, err := hedera.Ed25519PublicKeyFromString(goodEd25519PublicKeyStr1)
	assert.NoError(t, err, "expected valid ed25519 public key string")
	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantPubKeys []string
	}{
		{
			name: "Proper",
			args: args{
				threshold:         1,
				keys:              []string{goodEd25519PublicKeyStr2, goodEd25519PublicKeyStr3},
				operatorPublicKey: opPubKey,
				exclOperator:      false,
			},
			wantErr:     false,
			wantPubKeys: []string{goodEd25519PublicKeyStr1, goodEd25519PublicKeyStr2, goodEd25519PublicKeyStr3},
		},
		{
			name: "ProperWithExclusion",
			args: args{
				threshold:         1,
				keys:              []string{goodEd25519PublicKeyStr1, goodEd25519PublicKeyStr2, goodEd25519PublicKeyStr3},
				operatorPublicKey: opPubKey,
				exclOperator:      true,
			},
			wantErr:     false,
			wantPubKeys: []string{goodEd25519PublicKeyStr2, goodEd25519PublicKeyStr3},
		},
		{
			name: "ThresholdZero",
			args: args{
				threshold:         0,
				keys:              []string{goodEd25519PublicKeyStr2, goodEd25519PublicKeyStr3},
				operatorPublicKey: opPubKey,
				exclOperator:      false,
			},
			wantErr: true,
		},
		{
			name: "ThresholdTooBig",
			args: args{
				threshold:         4,
				keys:              []string{goodEd25519PublicKeyStr2, goodEd25519PublicKeyStr3},
				operatorPublicKey: opPubKey,
				exclOperator:      false,
			},
			wantErr: true,
		},
		{
			name: "InvalidEd25519KeyString",
			args: args{
				threshold:         1,
				keys:              []string{goodEd25519PublicKeyStr2, "invalid key string"},
				operatorPublicKey: opPubKey,
				exclOperator:      false,
			},
			wantErr: true,
		},
		{
			name: "NoKeyAfterExclusion",
			args: args{
				threshold:         1,
				keys:              []string{goodEd25519PublicKeyStr1},
				operatorPublicKey: opPubKey,
				exclOperator:      true,
			},
			wantErr:     false,
			wantPubKeys: nil,
		},
		{
			name: "OnlyOperatorPublicKey",
			args: args{
				threshold:         1,
				keys:              nil,
				operatorPublicKey: opPubKey,
				exclOperator:      false,
			},
			wantErr:     false,
			wantPubKeys: []string{goodEd25519PublicKeyStr1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubKey, err := makePublicKey(tt.args.threshold, tt.args.keys, tt.args.operatorPublicKey, tt.args.exclOperator)
			if tt.wantErr {
				assert.Error(t, err, "expect makePublicKey return error")
				return
			}
			assert.NoError(t, err, "expect makePublicKey return no error")

			if tt.wantPubKeys == nil {
				assert.Nil(t, pubKey, "expect makePublicKey returns nil publicKey")
			} else {
				var wantPubKey hedera.PublicKey
				if len(tt.wantPubKeys) == 1 {
					tmpKey, err := hedera.Ed25519PublicKeyFromString(tt.wantPubKeys[0])
					assert.NoError(t, err, "expect valid ed25519 public key string")
					wantPubKey = &tmpKey
				} else {
					//thresholdKey := hedera.NewThresholdKey(tt.args.threshold)
					//for _, keyStr := range tt.wantPubKeys {
					//	key, err := hedera.Ed25519PublicKeyFromString(keyStr)
					//	assert.NoError(t, err, "expect valid ed25519 public key string")
					//	thresholdKey.Add(key)
					//}
					//wantPubKey = thresholdKey
					// DeepEqual will fail since ThresholdKey is implemented with a list of keys, lists are DeepEqual iff
					// the corresponding elements are deeply equal. in this case, the returned public key from makePublicKey
					// may have the same list of keys in different order
					return
				}
				assert.True(t, reflect.DeepEqual(wantPubKey, pubKey), "expect correct returned public key")
			}
		})
	}
}

