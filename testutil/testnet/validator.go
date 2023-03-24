package testnet

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmttypes "github.com/cometbft/cometbft/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type ValidatorPrivKeys []*ValidatorPrivKey

type ValidatorPrivKey struct {
	Val cmted25519.PrivKey
	Del *secp256k1.PrivKey
}

func NewValidatorPrivKeys(n int) ValidatorPrivKeys {
	vpk := make(ValidatorPrivKeys, n)

	for i := range vpk {
		vpk[i] = &ValidatorPrivKey{
			Val: cmted25519.GenPrivKey(),
			Del: secp256k1.GenPrivKey(),
		}
	}

	return vpk
}

func (vpk ValidatorPrivKeys) CometGenesisValidators() CometGenesisValidators {
	cgv := make(CometGenesisValidators, len(vpk))

	for i, pk := range vpk {
		pubKey := pk.Val.PubKey()

		const votingPower = 1
		cmtVal := cmttypes.NewValidator(pubKey, votingPower)

		cgv[i] = &CometGenesisValidator{
			V: cmttypes.GenesisValidator{
				Address: cmtVal.Address,
				PubKey:  cmtVal.PubKey,
				Power:   cmtVal.VotingPower,
				Name:    fmt.Sprintf("val-%d", i),
			},
			PK: pk,
		}
	}

	return cgv
}

type CometGenesisValidators []*CometGenesisValidator

type CometGenesisValidator struct {
	V  cmttypes.GenesisValidator
	PK *ValidatorPrivKey
}

func (cgv CometGenesisValidators) ToComet() []cmttypes.GenesisValidator {
	vs := make([]cmttypes.GenesisValidator, len(cgv))
	for i, v := range cgv {
		vs[i] = v.V
	}
	return vs
}

func (cgv CometGenesisValidators) StakingValidators() (vals StakingValidators, supply sdk.Coins) {
	vals = make(StakingValidators, len(cgv))
	for i, v := range cgv {
		pk, err := cryptocodec.FromCmtPubKeyInterface(v.V.PubKey)
		if err != nil {
			panic(fmt.Errorf("failed to extract comet pub key: %w", err))
		}

		pkAny, err := codectypes.NewAnyWithValue(pk)
		if err != nil {
			panic(fmt.Errorf("failed to wrap pub key in any type: %w", err))
		}

		vals[i] = &StakingValidator{
			V: stakingtypes.Validator{
				OperatorAddress:   sdk.ValAddress(v.V.Address).String(), // TODO: this relies on global bech32 config.
				ConsensusPubkey:   pkAny,
				Status:            stakingtypes.Bonded,
				Tokens:            sdk.DefaultPowerReduction,
				DelegatorShares:   sdkmath.LegacyOneDec(),
				MinSelfDelegation: sdkmath.ZeroInt(),

				// more fields uncopied from testutil/sims/app_helpers.go:220
			},
			PK: v.PK,
		}

		supply = supply.Add(sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction))
	}

	return vals, supply
}

type StakingValidators []*StakingValidator

type StakingValidator struct {
	V  stakingtypes.Validator
	PK *ValidatorPrivKey
}

func (sv StakingValidators) ToStakingType() []stakingtypes.Validator {
	vs := make([]stakingtypes.Validator, len(sv))
	for i, v := range sv {
		vs[i] = v.V
	}
	return vs
}

func (sv StakingValidators) BondedPoolBalance() banktypes.Balance {
	var coins sdk.Coins

	for _, v := range sv {
		coins = coins.Add(sdk.NewCoin(sdk.DefaultBondDenom, v.V.Tokens))
	}

	return banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   coins,
	}
}

func (sv StakingValidators) BaseAccounts() BaseAccounts {
	ba := make(BaseAccounts, len(sv))

	for i, v := range sv {
		const accountNumber = 0
		const sequenceNumber = 0

		pubKey := v.PK.Del.PubKey()
		bech, err := bech32.ConvertAndEncode("cosmos", pubKey.Address().Bytes())
		if err != nil {
			panic(err)
		}
		accAddr, err := sdk.AccAddressFromBech32(bech)
		if err != nil {
			panic(err)
		}
		ba[i] = authtypes.NewBaseAccount(
			accAddr, pubKey, accountNumber, sequenceNumber,
		)
	}

	return ba
}

func (sv StakingValidators) Balances() []banktypes.Balance {
	bals := make([]banktypes.Balance, len(sv))

	for i, v := range sv {
		addr, err := bech32.ConvertAndEncode("cosmos", v.PK.Del.PubKey().Address().Bytes())
		if err != nil {
			panic(err)
		}
		bals[i] = banktypes.Balance{
			Address: addr,
			Coins:   sdk.Coins{sdk.NewCoin(sdk.DefaultBondDenom, v.V.Tokens)},
		}
	}

	return bals
}
