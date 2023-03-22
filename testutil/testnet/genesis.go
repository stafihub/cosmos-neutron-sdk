package testnet

import (
	"encoding/json"
	"fmt"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type GenesisBuilder struct {
	amino *codec.LegacyAmino
	codec *codec.ProtoCodec

	chainID string

	outer, appState map[string]json.RawMessage

	gentxs []sdk.Tx
}

func NewGenesisBuilder() *GenesisBuilder {
	ir := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(ir)
	stakingtypes.RegisterInterfaces(ir)
	banktypes.RegisterInterfaces(ir)
	authtypes.RegisterInterfaces(ir)
	pCodec := codec.NewProtoCodec(ir)

	return &GenesisBuilder{
		amino: codec.NewLegacyAmino(),
		codec: pCodec,

		outer:    map[string]json.RawMessage{},
		appState: map[string]json.RawMessage{},
	}
}

func (b *GenesisBuilder) GenTx(privVal secp256k1.PrivKey, val cmttypes.GenesisValidator, amount sdk.Coin) *GenesisBuilder {
	if b.chainID == "" {
		panic(fmt.Errorf("(*GenesisBuilder).GenTx called before (*GenesisBuilder).ChainID"))
	}

	pubKey, err := cryptocodec.FromCmtPubKeyInterface(val.PubKey)
	if err != nil {
		panic(err)
	}

	// Produce the create validator message.
	msg, err := stakingtypes.NewMsgCreateValidator(
		val.Address.Bytes(),
		pubKey,
		amount,
		stakingtypes.Description{
			Moniker: "TODO",
		},
		stakingtypes.CommissionRates{
			Rate:          sdk.MustNewDecFromStr("0.1"),
			MaxRate:       sdk.MustNewDecFromStr("0.2"),
			MaxChangeRate: sdk.MustNewDecFromStr("0.01"),
		},
		sdk.OneInt(),
	)
	if err != nil {
		panic(err)
	}
	valAddr, err := sdk.ValAddressFromBech32(msg.ValidatorAddress)
	if err != nil {
		panic(err)
	}

	msg.DelegatorAddress = sdk.AccAddress(valAddr).String()

	if err := msg.ValidateBasic(); err != nil {
		panic(err)
	}

	txConf := authtx.NewTxConfig(b.codec, tx.DefaultSignModes)

	txb := txConf.NewTxBuilder()
	if err := txb.SetMsgs(msg); err != nil {
		panic(err)
	}

	const signMode = signing.SignMode_SIGN_MODE_DIRECT

	// Generate bytes to be signed.
	bytesToSign, err := txConf.SignModeHandler().GetSignBytes(
		signing.SignMode_SIGN_MODE_DIRECT,
		authsigning.SignerData{
			ChainID: b.chainID,
			PubKey:  privVal.PubKey(),
			Address: sdk.MustBech32ifyAddressBytes("cosmos1", privVal.PubKey().Address()), // TODO: don't hardcode cosmos1!

			// No account or sequence number for gentx.
		},
		txb.GetTx(),
	)
	if err != nil {
		panic(err)
	}

	// Produce the signature.
	signed, err := privVal.Sign(bytesToSign)
	if err != nil {
		panic(err)
	}

	// Set the signature on the builder.
	if err := txb.SetSignatures(
		signing.SignatureV2{
			PubKey: privVal.PubKey(),
			Data: &signing.SingleSignatureData{
				SignMode:  signMode,
				Signature: signed,
			},
		},
	); err != nil {
		panic(err)
	}

	b.gentxs = append(b.gentxs, txb.GetTx())

	return b
}

func (b *GenesisBuilder) ChainID(id string) *GenesisBuilder {
	b.chainID = id

	var err error
	b.outer["chain_id"], err = json.Marshal(id)
	if err != nil {
		panic(err)
	}

	return b
}

func (b *GenesisBuilder) AuthParams(params authtypes.Params) *GenesisBuilder {
	j, err := json.Marshal(map[string]any{
		"params": params,
	})
	if err != nil {
		panic(err)
	}

	b.appState[authtypes.ModuleName] = j
	return b
}

func (b *GenesisBuilder) DefaultAuthParams() *GenesisBuilder {
	return b.AuthParams(authtypes.DefaultParams())
}

func (b *GenesisBuilder) Consensus(params *cmttypes.ConsensusParams, vals CometGenesisValidators) *GenesisBuilder {
	if params == nil {
		params = cmttypes.DefaultConsensusParams()
	}

	var err error
	b.outer[consensusparamtypes.ModuleName], err = (&genutiltypes.ConsensusGenesis{
		Params:     params,
		Validators: vals.ToComet(),
	}).MarshalJSON()
	if err != nil {
		panic(err)
	}

	return b
}

func (b *GenesisBuilder) StakingWithDefaultParams(vals StakingValidators, delegations []stakingtypes.Delegation) *GenesisBuilder {
	return b.Staking(stakingtypes.DefaultParams(), vals, delegations)
}

func (b *GenesisBuilder) Staking(
	params stakingtypes.Params,
	vals StakingValidators,
	delegations []stakingtypes.Delegation,
) *GenesisBuilder {
	var err error
	b.appState[stakingtypes.ModuleName], err = b.codec.MarshalJSON(
		stakingtypes.NewGenesisState(params, vals.ToStakingType(), delegations),
	)
	if err != nil {
		panic(err)
	}

	// Modify bank state for bonded pool.

	var coins sdk.Coins
	for _, v := range vals {
		coins = coins.Add(sdk.NewCoin(sdk.DefaultBondDenom, v.V.Tokens))
	}

	bondedPoolBalance := banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   coins,
	}

	// get bank types genesis, add account

	bankGenesis := banktypes.GetGenesisStateFromAppState(b.codec, b.appState)
	bankGenesis.Balances = append(bankGenesis.Balances, bondedPoolBalance)

	jBankGenesis, err := b.codec.MarshalJSON(bankGenesis)
	if err != nil {
		panic(err)
	}
	b.appState[banktypes.ModuleName] = jBankGenesis

	return b
}

func (b *GenesisBuilder) BankingWithDefaultParams(
	balances []banktypes.Balance,
	totalSupply sdk.Coins,
	denomMetadata []banktypes.Metadata,
	sendEnabled []banktypes.SendEnabled,
) *GenesisBuilder {
	return b.Banking(
		banktypes.DefaultParams(),
		balances,
		totalSupply,
		denomMetadata,
		sendEnabled,
	)
}

func (b *GenesisBuilder) Banking(
	params banktypes.Params,
	balances []banktypes.Balance,
	totalSupply sdk.Coins,
	denomMetadata []banktypes.Metadata,
	sendEnabled []banktypes.SendEnabled,
) *GenesisBuilder {
	var err error
	b.appState[banktypes.ModuleName], err = b.codec.MarshalJSON(
		banktypes.NewGenesisState(
			params,
			balances,
			totalSupply,
			denomMetadata,
			sendEnabled,
		),
	)
	if err != nil {
		panic(err)
	}
	return b
}

func (b *GenesisBuilder) BaseAccounts(ba BaseAccounts, balances []banktypes.Balance) *GenesisBuilder {
	// Logic mostly copied from AddGenesisAccount.

	authGenState := authtypes.GetGenesisStateFromAppState(b.codec, b.appState)
	bankGenState := banktypes.GetGenesisStateFromAppState(b.codec, b.appState)

	accs, err := authtypes.UnpackAccounts(authGenState.Accounts)
	if err != nil {
		panic(err)
	}

	for _, a := range ba {
		accs = append(accs, a)
	}
	accs = authtypes.SanitizeGenesisAccounts(accs)

	genAccs, err := authtypes.PackAccounts(accs)
	if err != nil {
		panic(err)
	}

	authGenState.Accounts = genAccs
	jAuthGenState, err := b.codec.MarshalJSON(&authGenState)
	if err != nil {
		panic(err)
	}
	b.appState[authtypes.ModuleName] = jAuthGenState

	bankGenState.Balances = append(bankGenState.Balances, balances...)
	bankGenState.Balances = banktypes.SanitizeGenesisBalances(bankGenState.Balances)

	jBankState, err := b.codec.MarshalJSON(bankGenState)
	if err != nil {
		panic(err)
	}
	b.appState[banktypes.ModuleName] = jBankState
	return b
}

func (b *GenesisBuilder) JSON() map[string]json.RawMessage {
	gentxGenesisState := genutiltypes.NewGenesisStateFromTx(
		authtx.NewTxConfig(b.codec, tx.DefaultSignModes).TxJSONEncoder(),
		b.gentxs,
	)

	b.appState = genutiltypes.SetGenesisStateInAppState(
		b.codec, b.appState, gentxGenesisState,
	)

	appState, err := b.amino.MarshalJSON(b.appState)
	if err != nil {
		panic(err)
	}

	b.outer["app_state"] = appState

	return b.outer
}

func (b *GenesisBuilder) Encode() []byte {
	j, err := b.amino.MarshalJSON(b.JSON())
	if err != nil {
		panic(err)
	}

	return j
}
