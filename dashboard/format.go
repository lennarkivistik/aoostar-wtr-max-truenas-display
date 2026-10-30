package dashboard

import "fmt"

type speedParts struct {
	Value string
	Unit  string
}

// formatSpeedParts formats a speed value with hysteresis to prevent unit
// flicker at boundaries. Once a unit is entered, the value must drop below
// 80% of the boundary to switch back down (or rise above the boundary to go up).
func formatSpeedParts(bytesPerSec float64, lastUnit *string) speedParts {
	bitsPerSec := bytesPerSec * 8
	const (
		Kbps       = 1000
		Mbps       = Kbps * 1000
		Gbps       = Mbps * 1000
		hysteresis = 0.8
	)

	unit := "b/s"
	switch {
	case bitsPerSec >= Gbps:
		unit = "Gb/s"
	case bitsPerSec >= Mbps:
		unit = "Mb/s"
	case bitsPerSec >= Kbps:
		unit = "Kb/s"
	}

	// Apply hysteresis: if the previous unit was higher, stay there until
	// the value drops decisively below the threshold.
	if *lastUnit != "" && *lastUnit != unit {
		switch *lastUnit {
		case "Gb/s":
			if bitsPerSec >= float64(Gbps)*hysteresis {
				unit = "Gb/s"
			}
		case "Mb/s":
			if unit == "Kb/s" && bitsPerSec >= float64(Mbps)*hysteresis {
				unit = "Mb/s"
			}
		case "Kb/s":
			if unit == "b/s" && bitsPerSec >= float64(Kbps)*hysteresis {
				unit = "Kb/s"
			}
		}
	}
	*lastUnit = unit

	switch unit {
	case "Gb/s":
		return speedParts{fmt.Sprintf("%.2f", bitsPerSec/Gbps), unit}
	case "Mb/s":
		return speedParts{fmt.Sprintf("%.1f", bitsPerSec/Mbps), unit}
	case "Kb/s":
		return speedParts{fmt.Sprintf("%.0f", bitsPerSec/Kbps), unit}
	default:
		return speedParts{fmt.Sprintf("%.0f", bitsPerSec), unit}
	}
}
