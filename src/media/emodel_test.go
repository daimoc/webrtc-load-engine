package media

import (
	"math"
	"testing"
	"time"
)

// Reference values computed by hand from the formulas of research R4:
// R = 93.2 − Id − Ie,eff, Id = 0.024·Ta + 0.11·(Ta − 177.3)·H(Ta − 177.3),
// Ie,eff = 95·Ppl/(Ppl + 25.1), MOS = 1 + 0.035R + R(R − 60)(100 − R)·7e-6.
func TestRFactorAndMOS(t *testing.T) {
	cases := []struct {
		ta  time.Duration
		ppl float64
		r   float64
		mos float64
	}{
		{0, 0, 93.20, 4.409},
		{100 * time.Millisecond, 1, 87.16, 4.263},
		{200 * time.Millisecond, 5, 70.12, 3.603},
		{400 * time.Millisecond, 0, 59.10, 3.053},
		{80 * time.Millisecond, 2, 84.27, 4.175},
	}
	for _, c := range cases {
		if r := RFactor(c.ppl, c.ta); math.Abs(r-c.r) > 0.01 {
			t.Errorf("RFactor(%g, %s) = %.3f, want %.2f", c.ppl, c.ta, r, c.r)
		}
		if m := MOS(c.ppl, c.ta); math.Abs(m-c.mos) > 0.01 {
			t.Errorf("MOS(%g, %s) = %.3f, want %.3f", c.ppl, c.ta, m, c.mos)
		}
	}
}

func TestMOS_Bounds(t *testing.T) {
	if m := mosFromR(-5); m != 1 {
		t.Errorf("R < 0: %g, want 1", m)
	}
	if m := mosFromR(120); m != 4.5 {
		t.Errorf("R > 100: %g, want 4.5", m)
	}
	if m := MOS(100, 2*time.Second); m < 1 || m > 1.1 {
		t.Errorf("worst case %g", m)
	}
	if m, want := MOS(-1, -time.Second), MOS(0, 0); m != want {
		t.Errorf("negative inputs: %g, want %g", m, want)
	}
	if m, want := MOS(math.NaN(), 0), MOS(0, 0); m != want {
		t.Errorf("NaN loss: %g, want %g", m, want)
	}
}
