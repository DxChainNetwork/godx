package storagehostmanager

const (
	priceFloor                = float64(0.1)
	depositFloor              = priceFloor * 2
	depositExponentialSmall   = 4
	depositExponentialLarge   = 0.5
	interactionExponentiation = 10
)

const (
	minStorage = uint64(20e9)
)
