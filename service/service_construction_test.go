package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ava-labs/avalanche-rosetta/mapper"
	mocks "github.com/ava-labs/avalanche-rosetta/mocks/client"
	"github.com/ava-labs/coreth/interfaces"

	"github.com/coinbase/rosetta-sdk-go/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

const (
	defaultSymbol          = "TEST"
	defaultDecimals        = 18
	defaultContractAddress = "0x30e5449b6712Adf4156c8c474250F6eA4400eB82"
	defaultFromAddress     = "0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"
	defaultToAddress       = "0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d"
)

func TestConstructionMetadata(t *testing.T) {
	client := &mocks.Client{}
	ctx := context.Background()
	service := ConstructionService{
		config: &Config{Mode: ModeOnline},
		client: client,
	}

	t.Run("unavailable in offline mode", func(t *testing.T) {
		service := ConstructionService{
			config: &Config{
				Mode: ModeOffline,
			},
		}

		resp, err := service.ConstructionMetadata(
			context.Background(),
			&types.ConstructionMetadataRequest{},
		)
		assert.Nil(t, resp)
		assert.Equal(t, errUnavailableOffline.Code, err.Code)
	})

	t.Run("requires from address", func(t *testing.T) {
		resp, err := service.ConstructionMetadata(
			context.Background(),
			&types.ConstructionMetadataRequest{},
		)
		assert.Nil(t, resp)
		assert.Equal(t, errInvalidInput.Code, err.Code)
		assert.Equal(t, "from address is not provided", err.Details["error"])
	})

	t.Run("basic native transfer", func(t *testing.T) {
		to := common.HexToAddress(defaultToAddress)
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From:  common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:    &to,
				Value: big.NewInt(42894881044106498),
			},
		).Return(
			uint64(21001),
			nil,
		).Once()
		var input = map[string]interface{}{"from": "0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309", "to": "0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d", "value": "0x9864aac3510d02"} //nolint:lll
		resp, err := service.ConstructionMetadata(
			ctx,
			&types.ConstructionMetadataRequest{
				Options: input,
			},
		)
		assert.Nil(t, err)
		metadata := &metadata{
			GasPrice: big.NewInt(1000000000),
			GasLimit: 21_001,
			Nonce:    0,
		}
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "21001000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, resp)
	})
	t.Run("basic erc20 transfer", func(t *testing.T) {
		contractAddress := common.HexToAddress(defaultContractAddress)
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress(defaultFromAddress),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From: common.HexToAddress(defaultFromAddress),
				To:   &contractAddress,
				Data: common.Hex2Bytes("a9059cbb000000000000000000000000920eb8ca79f07eb3bfc39c324c8113948ed3104c00000000000000000000000000000000000000000000000000000000b4d360e3"), //nolint:lll
			},
		).Return(
			uint64(21001),
			nil,
		).Once()
		var currencyMetadata = map[string]interface{}{
			"contractAddress": defaultContractAddress,
		}
		var currency = map[string]interface{}{
			"symbol":   defaultSymbol,
			"decimals": defaultDecimals,
			"metadata": currencyMetadata,
		}
		var input = map[string]interface{}{"from": defaultFromAddress, "to": "0x920eb8ca79f07eb3bfc39c324c8113948ed3104c", "value": "0xb4d360e3", "currency": currency} //nolint:lll
		resp, err := service.ConstructionMetadata(
			ctx,
			&types.ConstructionMetadataRequest{
				Options: input,
			},
		)
		assert.Nil(t, err)
		metadata := &metadata{
			GasPrice: big.NewInt(1000000000),
			GasLimit: 21_001,
			Nonce:    0,
		}
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "21001000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, resp)
	})
}

func TestContructionHash(t *testing.T) {
	service := ConstructionService{}

	t.Run("no transaction", func(t *testing.T) {
		resp, err := service.ConstructionHash(
			context.Background(),
			&types.ConstructionHashRequest{},
		)
		assert.Nil(t, resp)
		assert.Equal(t, errInvalidInput.Code, err.Code)
		assert.Equal(t, "signed transaction value is not provided", err.Details["error"])
	})

	t.Run("invalid transaction", func(t *testing.T) {
		resp, err := service.ConstructionHash(context.Background(), &types.ConstructionHashRequest{
			SignedTransaction: "{}",
		})
		assert.Nil(t, resp)
		assert.Equal(t, errInvalidInput.Code, err.Code)
	})

	t.Run("valid transaction", func(t *testing.T) {
		signed := `{"nonce":"0x6","gasPrice":"0x6d6e2edc00","gas":"0x5208","to":"0x85ad9d1fcf50b72255e4288dca0ad29f5f509409","value":"0xde0b6b3a7640000","input":"0x","v":"0x150f6","r":"0x64d46cc17cbdbcf73b204a6979172eb3148237ecd369181b105e92b0d7fa49a7","s":"0x285063de57245f532a14b13f605bed047a9d20ebfd0db28e01bc8cc9eaac40ee","hash":"0x92ea9280c1653aa9042c7a4d3a608c2149db45064609c18b270c7c73738e2a46"}` //nolint:lll
		request := signedTransactionWrapper{SignedTransaction: []byte(signed), Currency: nil}

		json, marshalErr := json.Marshal(request)
		assert.Nil(t, marshalErr)

		resp, err := service.ConstructionHash(context.Background(), &types.ConstructionHashRequest{
			SignedTransaction: string(json),
		})
		assert.Nil(t, err)
		assert.Equal(
			t,
			"0x92ea9280c1653aa9042c7a4d3a608c2149db45064609c18b270c7c73738e2a46",
			resp.TransactionIdentifier.Hash,
		)
	})
}

func TestConstructionDerive(t *testing.T) {
	service := ConstructionService{}

	t.Run("no public key", func(t *testing.T) {
		resp, err := service.ConstructionDerive(
			context.Background(),
			&types.ConstructionDeriveRequest{},
		)
		assert.Nil(t, resp)
		assert.Equal(t, errInvalidInput.Code, err.Code)
		assert.Equal(t, "public key is not provided", err.Details["error"])
	})

	t.Run("invalid public key", func(t *testing.T) {
		resp, err := service.ConstructionDerive(
			context.Background(),
			&types.ConstructionDeriveRequest{
				PublicKey: &types.PublicKey{
					Bytes:     []byte("invaliddata"),
					CurveType: types.Secp256k1,
				},
			},
		)
		assert.Nil(t, resp)
		assert.Equal(t, errInvalidInput.Code, err.Code)
		assert.Equal(t, "invalid public key", err.Details["error"])
	})

	t.Run("valid public key", func(t *testing.T) {
		src := "03d0156cec2e01eff9c66e5dbc3c70f98214ec90a25eb43320ebcddc1a94b677f0"
		b, _ := hex.DecodeString(src)

		resp, err := service.ConstructionDerive(
			context.Background(),
			&types.ConstructionDeriveRequest{
				PublicKey: &types.PublicKey{
					Bytes:     b,
					CurveType: types.Secp256k1,
				},
			},
		)
		assert.Nil(t, err)
		assert.Equal(
			t,
			"0x156daFC6e9A1304fD5C9AB686acB4B3c802FE3f7",
			resp.AccountIdentifier.Address,
		)
	})
}

func forceMarshalMap(t *testing.T, i interface{}) map[string]interface{} {
	m, err := marshalJSONMap(i)
	if err != nil {
		t.Fatalf("could not marshal map %s", types.PrintStruct(i))
	}

	return m
}

func TestPreprocessMetadata(t *testing.T) {
	ctx := context.Background()
	client := &mocks.Client{}
	networkIdentifier := &types.NetworkIdentifier{
		Network:    "Fuji",
		Blockchain: "Avalanche",
	}
	service := ConstructionService{
		config: &Config{Mode: ModeOnline},
		client: client,
	}
	intent := `[{"operation_identifier":{"index":0},"type":"CALL","account":{"address":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"},"amount":{"value":"-42894881044106498","currency":{"symbol":"AVAX","decimals":18}}},{"operation_identifier":{"index":1},"type":"CALL","account":{"address":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d"},"amount":{"value":"42894881044106498","currency":{"symbol":"AVAX","decimals":18}}}]` //nolint
	t.Run("currency info doesn't match between the operations", func(t *testing.T) {
		unclear_intent := `[{"operation_identifier":{"index":0},"type":"CALL","account":{"address":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"},"amount":{"value":"-42894881044106498","currency":{"symbol":"AVAX","decimals":18}}},{"operation_identifier":{"index":1},"type":"CALL","account":{"address":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d"},"amount":{"value":"42894881044106498","currency":{"symbol":"NOAX","decimals":18}}}]` //nolint

		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(unclear_intent), &ops))
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier: networkIdentifier,
				Operations:        ops,
			},
		)
		assert.Nil(t, preprocessResponse)
		assert.Equal(t, "currency info doesn't match between the operations", err.Details["error"])
	})
	t.Run("basic flow", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier: networkIdentifier,
				Operations:        ops,
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02", "currency":{"symbol":"AVAX","decimals":18}}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1000000000),
			GasLimit: 21_001,
			Nonce:    0,
		}

		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		to := common.HexToAddress("0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d")
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From:  common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:    &to,
				Value: big.NewInt(42894881044106498),
			},
		).Return(
			uint64(21001),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "21001000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("basic flow (backwards compatible)", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))

		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))

		metadata := &metadata{
			GasPrice: big.NewInt(1000000000),
			GasLimit: 21_000,
			Nonce:    0,
		}

		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "21000000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("custom gas price flow", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier: networkIdentifier,
				Operations:        ops,
				Metadata: map[string]interface{}{
					"gas_price": "1100000000",
				},
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02","gas_price":"0x4190ab00", "currency":{"decimals":18, "symbol":"AVAX"}}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1100000000),
			GasLimit: 21_000,
			Nonce:    0,
		}

		to := common.HexToAddress("0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d")
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From:  common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:    &to,
				Value: big.NewInt(42894881044106498),
			},
		).Return(
			uint64(21000),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "23100000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("custom gas price flow (ignore multiplier)", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))
		multiplier := float64(1.1)
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier:      networkIdentifier,
				Operations:             ops,
				SuggestedFeeMultiplier: &multiplier,
				Metadata: map[string]interface{}{
					"gas_price": "1100000000",
				},
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02","gas_price":"0x4190ab00","suggested_fee_multiplier":1.1, "currency":{"decimals":18, "symbol":"AVAX"}}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1100000000),
			GasLimit: 21_000,
			Nonce:    0,
		}

		to := common.HexToAddress("0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d")
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From:  common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:    &to,
				Value: big.NewInt(42894881044106498),
			},
		).Return(
			uint64(21000),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "23100000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("fee multiplier", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))
		multiplier := float64(1.1)
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier:      networkIdentifier,
				Operations:             ops,
				SuggestedFeeMultiplier: &multiplier,
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02","suggested_fee_multiplier":1.1, "currency":{"decimals":18, "symbol":"AVAX"}}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1100000000),
			GasLimit: 21_000,
			Nonce:    0,
		}

		to := common.HexToAddress("0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d")
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From:  common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:    &to,
				Value: big.NewInt(42894881044106498),
			},
		).Return(
			uint64(21000),
			nil,
		).Once()
		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "23100000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("custom nonce", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))
		multiplier := float64(1.1)
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier:      networkIdentifier,
				Operations:             ops,
				SuggestedFeeMultiplier: &multiplier,
				Metadata: map[string]interface{}{
					"nonce": "1",
				},
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02","suggested_fee_multiplier":1.1, "nonce":"0x1", "currency":{"decimals":18, "symbol":"AVAX"}}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1100000000),
			GasLimit: 21_000,
			Nonce:    1,
		}

		to := common.HexToAddress("0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d")
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From:  common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:    &to,
				Value: big.NewInt(42894881044106498),
			},
		).Return(
			uint64(21000),
			nil,
		).Once()
		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "23100000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("custom gas limit", func(t *testing.T) {
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(intent), &ops))
		multiplier := float64(1.1)
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier:      networkIdentifier,
				Operations:             ops,
				SuggestedFeeMultiplier: &multiplier,
				Metadata: map[string]interface{}{
					"gas_limit": "40000",
				},
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02","suggested_fee_multiplier":1.1,"gas_limit":"0x9c40", "currency":{"decimals":18, "symbol":"AVAX"}}` //nolint:lll
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1100000000),
			GasLimit: 40_000,
			Nonce:    0,
		}

		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()
		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "44000000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})

	t.Run("basic erc20 flow", func(t *testing.T) {
		erc20_intent := `[{"operation_identifier":{"index":0},"type":"ERC20_TRANSFER","account":{"address":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"},"amount":{"value":"-42894881044106498","currency":{"symbol":"TEST","decimals":18, "metadata": {"contractAddress": "0x30e5449b6712Adf4156c8c474250F6eA4400eB82"}}}},{"operation_identifier":{"index":1},"type":"ERC20_TRANSFER","account":{"address":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d"},"amount":{"value":"42894881044106498","currency":{"symbol":"TEST","decimals":18, "metadata": {"contractAddress": "0x30e5449b6712Adf4156c8c474250F6eA4400eB82"}}}}]` //nolint
		tokenList := []string{defaultContractAddress}

		service := ConstructionService{
			config: &Config{Mode: ModeOnline, TokenWhiteList: tokenList},
			client: client,
		}
		currency := &types.Currency{Symbol: defaultSymbol, Decimals: defaultDecimals}
		client.On(
			"ContractInfo",
			common.HexToAddress(defaultContractAddress),
			true,
		).Return(
			currency,
			nil,
		).Once()
		var ops []*types.Operation
		assert.NoError(t, json.Unmarshal([]byte(erc20_intent), &ops))
		preprocessResponse, err := service.ConstructionPreprocess(
			ctx,
			&types.ConstructionPreprocessRequest{
				NetworkIdentifier: networkIdentifier,
				Operations:        ops,
			},
		)
		assert.Nil(t, err)
		optionsRaw := `{"from":"0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309","to":"0x57B414a0332B5CaB885a451c2a28a07d1e9b8a8d","value":"0x9864aac3510d02", "currency":{"symbol":"TEST","decimals":18, "metadata": {"contractAddress": "0x30e5449b6712Adf4156c8c474250F6eA4400eB82"}}}` //nolint
		var opt options
		assert.NoError(t, json.Unmarshal([]byte(optionsRaw), &opt))
		assert.Equal(t, &types.ConstructionPreprocessResponse{
			Options: forceMarshalMap(t, &opt),
		}, preprocessResponse)

		metadata := &metadata{
			GasPrice: big.NewInt(1000000000),
			GasLimit: 21_001,
			Nonce:    0,
		}

		client.On(
			"SuggestGasPrice",
			ctx,
		).Return(
			big.NewInt(1000000000),
			nil,
		).Once()
		contractAddress := common.HexToAddress(defaultContractAddress)
		client.On(
			"EstimateGas",
			ctx,
			interfaces.CallMsg{
				From: common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
				To:   &contractAddress,
				Data: common.Hex2Bytes("a9059cbb00000000000000000000000057B414a0332B5CaB885a451c2a28a07d1e9b8a8d000000000000000000000000000000000000000000000000009864aac3510d02"), //nolint:lll
			},
		).Return(
			uint64(21001),
			nil,
		).Once()
		client.On(
			"NonceAt",
			ctx,
			common.HexToAddress("0xe3a5B4d7f79d64088C8d4ef153A7DDe2B2d47309"),
			(*big.Int)(nil),
		).Return(
			uint64(0),
			nil,
		).Once()

		metadataResponse, err := service.ConstructionMetadata(ctx, &types.ConstructionMetadataRequest{
			NetworkIdentifier: networkIdentifier,
			Options:           forceMarshalMap(t, &opt),
		})
		assert.Nil(t, err)
		assert.Equal(t, &types.ConstructionMetadataResponse{
			Metadata: forceMarshalMap(t, metadata),
			SuggestedFee: []*types.Amount{
				{
					Value:    "21001000000000",
					Currency: mapper.AvaxCurrency,
				},
			},
		}, metadataResponse)
	})
}
