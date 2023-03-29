package testnettest

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/simapp"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/node"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/testutil/testnet"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

// Use a limited set of available ports to ensure that
// retries eventually land on a free port.
func TestCometStarter_PortContention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long test in short mode")
	}

	const nVals = 4

	// Find n+1 addresses that should be free.
	// Ephemeral port range should start at about 49k+
	// according to `sysctl net.inet.ip.portrange` on macOS,
	// and at about 32k+ on Linux
	// according to `sysctl net.ipv4.ip_local_port_range`.
	//
	// Because we attempt to find free addresses outside that range,
	// it is unlikely that another process will claim a port
	// we discover to be free, during the time this test runs.
	const portSeekStart = 19000
	reuseAddrs := make([]string, 0, nVals+1)
	for i := portSeekStart; i < portSeekStart+1000; i++ {
		addr := fmt.Sprintf("127.0.0.1:%d", i)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			// No need to log the failure.
			continue
		}

		// If the port was free, append it to our reusable addresses.
		reuseAddrs = append(reuseAddrs, "tcp://"+addr)
		_ = ln.Close()

		if len(reuseAddrs) == nVals+1 {
			break
		}
	}

	if len(reuseAddrs) != nVals+1 {
		t.Fatalf("needed %d free ports but only found %d", nVals+1, len(reuseAddrs))
	}

	// Now that we have one more port than the number of validators,
	// there is a good chance that picking a random port will conflict with a previously chosen one.
	// But since CometStarter retries several times,
	// it should eventually land on a free port.

	valPKs := testnet.NewValidatorPrivKeys(nVals)
	cmtVals := valPKs.CometGenesisValidators()
	stakingVals := cmtVals.StakingValidators()

	const chainID = "simapp-cometstarter"

	b := testnet.NewGenesisBuilder().
		ChainID(chainID).
		DefaultAuthParams().
		Consensus(nil, cmtVals).
		BaseAccounts(stakingVals.BaseAccounts(), nil).
		StakingWithDefaultParams(nil, nil).
		BankingWithDefaultParams(stakingVals.Balances(), nil, nil, nil).
		DefaultDistribution().
		DefaultMint().
		SlashingWithDefaultParams(nil, nil)

	for i, v := range valPKs {
		b.GenTx(*v.Del, cmtVals[i].V, sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction))
	}

	jGenesis := b.Encode()

	const nRuns = 4

	// Use an info-level logger, because the debug logs in comet are noisy
	// and there is a data race in comet debug logs,
	// due to be fixed in v0.37.1 which is not yet released:
	// https://github.com/cometbft/cometbft/pull/532
	logger := log.NewTestLoggerInfo(t)

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 0; i < nRuns; i++ {
		t.Run(fmt.Sprintf("attempt %d", i), func(t *testing.T) {
			nodes := make([]*node.Node, nVals)
			for j := 0; j < nVals; j++ {
				rootDir := t.TempDir()

				app := simapp.NewSimApp(
					logger.With("instance", j),
					dbm.NewMemDB(),
					nil,
					true,
					simtestutil.NewAppOptionsWithFlagHome(rootDir),
					baseapp.SetChainID(chainID),
				)

				cfg := cmtcfg.DefaultConfig()

				// With the default goleveldb backend,
				// we see spurious "resource temporarily unavailable" errors in this test, for some reason.
				// We aren't exercising anything with the comet database in this test,
				// so memdb suffices.
				cfg.BaseConfig.DBBackend = "memdb"

				cs := testnet.NewCometStarter(
					app,
					cfg,
					valPKs[j].Val,
					jGenesis,
					rootDir,
				).
					Logger(logger.With("rootmodule", fmt.Sprintf("comet_node-%d", j))).
					TCPAddrChooser(func() string {
						return reuseAddrs[rng.Intn(len(reuseAddrs))]
					})

				n, err := cs.Start()
				require.NoError(t, err)
				defer n.Stop()

				nodes[j] = n
				curNetAddress := n.PEXReactor().Switch.NetAddress()
				for k := 0; k < j; k++ {
					_ = nodes[k].PEXReactor().Switch.DialPeerWithAddress(curNetAddress)
					_ = n.PEXReactor().Switch.DialPeerWithAddress(nodes[k].PEXReactor().Switch.NetAddress())
				}
			}

			heightAdvanced := false
			for j := 0; j < 60; j++ {
				cs := nodes[0].ConsensusState()
				if cs.GetLastHeight() < 2 {
					time.Sleep(500 * time.Millisecond)
					continue
				}

				// Saw height advance.
				heightAdvanced = true
				break
			}

			if !heightAdvanced {
				t.Fatalf("consensus height did not advance in approximately 10 seconds")
			}

			// Ensure nodes are stopped completely,
			// so that we don't get t.Cleanup errors around directories not being empty.
			for _, n := range nodes {
				_ = n.Stop()
				n.Wait()
			}
		})
	}
}
