package testnet_test

import (
	"context"
	"testing"

	"cosmossdk.io/log"
	"cosmossdk.io/simapp"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/rpc/client/http"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/testutil/testnet"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// A single comet server in a network runs an RPC server successfully.
func TestCometRPC_SingleRPCServer(t *testing.T) {
	const nVals = 2

	valPKs := testnet.NewValidatorPrivKeys(nVals)
	cmtVals := valPKs.CometGenesisValidators()
	stakingVals := cmtVals.StakingValidators()

	const chainID = "comet-rpc-singleton"

	b := testnet.DefaultGenesisBuilderOnlyValidators(
		chainID,
		stakingVals,
		sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction),
	)

	jGenesis := b.Encode()

	// Logs shouldn't be necessary here because we are exercising CometStarter.
	logger := log.NewNopLogger()

	nodes, err := testnet.NewNetwork(nVals, func(idx int) *testnet.CometStarter {
		rootDir := t.TempDir()

		app := simapp.NewSimApp(
			logger,
			dbm.NewMemDB(),
			nil,
			true,
			simtestutil.NewAppOptionsWithFlagHome(rootDir),
			baseapp.SetChainID(chainID),
		)

		cfg := cmtcfg.DefaultConfig()
		cfg.BaseConfig.DBBackend = "memdb"

		cs := testnet.NewCometStarter(
			app,
			cfg,
			valPKs[idx].Val,
			jGenesis,
			rootDir,
		)

		// Only enable the RPC on the first service.
		if idx == 0 {
			cs = cs.RPCListen()
		}

		return cs
	})
	require.NoError(t, err)
	defer func() {
		_ = nodes.Stop()
		nodes.Wait()
	}()

	c, err := http.New(nodes[0].Config().RPC.ListenAddress, "/websocket")
	require.NoError(t, err)

	ctx := context.Background()
	st, err := c.Status(ctx)
	require.NoError(t, err)

	// Simple assertion to ensure we have a functioning RPC.
	require.Equal(t, chainID, st.NodeInfo.Network)
}

// Starting two comet instances with an RPC server,
// fails with a predictable error.
func TestCometRPC_MultipleRPCError(t *testing.T) {
	const nVals = 2

	valPKs := testnet.NewValidatorPrivKeys(nVals)
	cmtVals := valPKs.CometGenesisValidators()
	stakingVals := cmtVals.StakingValidators()

	const chainID = "comet-rpc-multiple"

	b := testnet.DefaultGenesisBuilderOnlyValidators(
		chainID,
		stakingVals,
		sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction),
	)

	jGenesis := b.Encode()

	// Logs shouldn't be necessary here because we are exercising CometStarter.
	logger := log.NewNopLogger()

	_, err := testnet.NewNetwork(nVals, func(idx int) *testnet.CometStarter {
		rootDir := t.TempDir()

		app := simapp.NewSimApp(
			logger,
			dbm.NewMemDB(),
			nil,
			true,
			simtestutil.NewAppOptionsWithFlagHome(rootDir),
			baseapp.SetChainID(chainID),
		)

		cfg := cmtcfg.DefaultConfig()
		cfg.BaseConfig.DBBackend = "memdb"

		return testnet.NewCometStarter(
			app,
			cfg,
			valPKs[idx].Val,
			jGenesis,
			rootDir,
		).RPCListen() // Every node has RPCListen enabled, which will cause a failure.
	})
	require.NoError(t, err)

	// Returned error is convertible to CometRPCInUseError.
	// We can't test the exact value because it includes a stack trace.
	require.ErrorAs(t, err, new(testnet.CometRPCInUseError))
}
