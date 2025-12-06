package models

// Embedded contract addresses
const (
	PlasmaAddress      = "z1qxemdeddedxplasmaxxxxxxxxxxxxxxxxsctrp"
	PillarAddress      = "z1qxemdeddedxpyllarxxxxxxxxxxxxxxxsy3fmg"
	TokenAddress       = "z1qxemdeddedxt0kenxxxxxxxxxxxxxxxxh9amk0"
	SentinelAddress    = "z1qxemdeddedxsentynelxxxxxxxxxxxxxwy0r2r"
	StakeAddress       = "z1qxemdeddedxstakexxxxxxxxxxxxxxxxjv8v62"
	AcceleratorAddress = "z1qxemdeddedxaccelerat0rxxxxxxxxxxp4tk22"
	SwapAddress        = "z1qxemdeddedxswapxxxxxxxxxxxxxxxxxxl4yww"
	LiquidityAddress   = "z1qxemdeddedxlyquydytyxxxxxxxxxxxxflaaae"
	BridgeAddress      = "z1qxemdeddedxdrydgexxxxxxxxxxxxxxxmqgr0d"
	HtlcAddress        = "z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw"
	SporkAddress       = "z1qxemdeddedxsp0rkxxxxxxxxxxxxxxxx956u48"
)

// Special addresses
const (
	EmptyAddress             = "z1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqsggv2f"
	LiquidityTreasuryAddress = "z1qqw8f3qxx9zg92xgckqdpfws3dw07d26afsj74"
)

// Token standards
const (
	EmptyTokenStandard = "zts1qqqqqqqqqqqqqqqqtq587y"
	ZnnTokenStandard   = "zts1znnxxxxxxxxxxxxx9z4ulx"
	QsrTokenStandard   = "zts1qsrxxxxxxxxxxxxxmrhjll"
)

// Genesis momentum timestamp (used to fetch first momentum)
const GenesisMomentumTime = 1637755210

// Fusion expiration time in seconds (1 hour)
const FusionExpirationTime = 3600

// EmbeddedContractAddresses returns all embedded contract addresses
func EmbeddedContractAddresses() []string {
	return []string{
		PlasmaAddress,
		PillarAddress,
		TokenAddress,
		SentinelAddress,
		StakeAddress,
		AcceleratorAddress,
		SwapAddress,
		LiquidityAddress,
		BridgeAddress,
		HtlcAddress,
		SporkAddress,
	}
}

// IsEmbeddedContract checks if an address is an embedded contract
func IsEmbeddedContract(address string) bool {
	for _, addr := range EmbeddedContractAddresses() {
		if addr == address {
			return true
		}
	}
	return false
}

// RewardContractAddresses returns contract addresses that distribute rewards
func RewardContractAddresses() []string {
	return []string{
		PillarAddress,
		SentinelAddress,
		StakeAddress,
		LiquidityAddress,
	}
}
