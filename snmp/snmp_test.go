package snmp

import (
	"math/big"
	"testing"
)

func TestCalculateCounterDiff(t *testing.T) {
	tests := []struct {
		lastValue    *big.Int
		currentValue *big.Int
		want         *big.Int
	}{
		{
			lastValue:    big.NewInt(4021302487),
			currentValue: big.NewInt(59461020),
			want:         big.NewInt(333125828),
		},
		{
			// a lastValue of 1000 rolling to 100 is probably due to counter reset due to equipment reboot or manual
			// counter clearing on the interface, if that's the case, it should assume a 32bit counter
			lastValue:    big.NewInt(1000),
			currentValue: big.NewInt(100),
			want:         big.NewInt(4294966395),
		},
		{
			lastValue:    new(big.Int).SetUint64(18446744073709551515),
			currentValue: big.NewInt(1000),
			want:         big.NewInt(1100),
		},
		{
			lastValue:    new(big.Int).SetUint64(4014566411),
			currentValue: big.NewInt(4114566411),
			want:         big.NewInt(100000000),
		},
		{
			lastValue:    new(big.Int).SetUint64(16446744073709551515),
			currentValue: new(big.Int).SetUint64(16446744073709551815),
			want:         big.NewInt(300),
		},
	}

	for _, test := range tests {
		got := CalculateCounterDiff(test.lastValue, test.currentValue)
		if got.Cmp(test.want) != 0 {
			t.Errorf("CalculateCounterDiff(%v, %v) = %v, want %v", test.lastValue, test.currentValue, got, test.want)
		}
	}
}
