package media

import (
	"math"
	"time"
)

// E-model (ITU-T G.107) values of research R4. G.113 gives no Opus values
// for the narrowband E-model, so the bots use those of G.711 with packet
// loss concealment (G.113 Appendix I) as a stand-in, as common E-model
// tools do: the best score is therefore 4.41.
const (
	eModelR0     = 93.2 // Basic signal-to-noise ratio minus default impairments
	eModelIe     = 0    // Equipment impairment of the codec
	eModelBpl    = 25.1 // Packet-loss robustness of the codec
	eModelBurstR = 1    // Random loss
)

// RFactor returns the E-model transmission rating for a loss (or late)
// percentage and a one-way mouth-to-ear delay.
func RFactor(lossPct float64, oneWayDelay time.Duration) float64 {
	if math.IsNaN(lossPct) || lossPct < 0 {
		lossPct = 0
	}
	if lossPct > 100 {
		lossPct = 100
	}
	ta := float64(oneWayDelay) / float64(time.Millisecond)
	if ta < 0 {
		ta = 0
	}
	id := 0.024 * ta
	if ta > 177.3 {
		id += 0.11 * (ta - 177.3)
	}
	ieEff := eModelIe + (95-eModelIe)*lossPct/(lossPct/eModelBurstR+eModelBpl)
	return eModelR0 - id - ieEff
}

// MOS returns the estimated listening score (1 to 4.5) for a loss (or late)
// percentage and a one-way mouth-to-ear delay.
func MOS(lossPct float64, oneWayDelay time.Duration) float64 {
	return mosFromR(RFactor(lossPct, oneWayDelay))
}

// mosFromR converts a rating to a score (G.107 Annex B).
func mosFromR(r float64) float64 {
	switch {
	case r <= 0:
		return 1
	case r >= 100:
		return 4.5
	}
	return 1 + 0.035*r + r*(r-60)*(100-r)*7e-6
}
