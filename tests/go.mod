module github.com/cosmos/cosmos-sdk/tests

go 1.19

require (
	cosmossdk.io/api v0.3.1
	cosmossdk.io/depinject v1.0.0-alpha.4
	cosmossdk.io/math v1.2.0
	cosmossdk.io/simapp v0.0.0-00010101000000-000000000000
	github.com/cometbft/cometbft v0.37.2
	github.com/cometbft/cometbft-db v0.8.0
	github.com/cosmos/cosmos-sdk v0.47.5
	github.com/cosmos/gogoproto v1.4.10
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.3.0
	github.com/spf13/cobra v1.6.1
	github.com/stretchr/testify v1.8.4
	google.golang.org/protobuf v1.31.0
	gotest.tools/v3 v3.5.0
	pgregory.net/rapid v0.5.5
)

replace (
	// We always want to test against the latest version of the simapp.
	cosmossdk.io/simapp => ../simapp
	github.com/99designs/keyring => github.com/cosmos/keyring v1.2.0
	// We always want to test against the latest version of the SDK.
	github.com/cosmos/cosmos-sdk => ../.
	// Fix upstream GHSA-h395-qcrw-5vmq and GHSA-3vp4-m3rf-835h vulnerabilities.
	// TODO Remove it: https://github.com/cosmos/cosmos-sdk/issues/10409
	github.com/gin-gonic/gin => github.com/gin-gonic/gin v1.9.0
)
