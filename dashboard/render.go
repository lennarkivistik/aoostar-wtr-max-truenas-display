package dashboard

import (
	"fmt"
	"image"
	"image/color"

	"go-wtr-display/display"
	"go-wtr-display/network"
)

// Layout constants for 960x376 display
const (
	headerHeight = 52
	speedsHeight = 210
	statsHeight  = 114
)

// Enhanced color palette
var (
	colorBackground = color.RGBA{15, 23, 42, 255}    // Dark navy background
	colorTeal       = color.RGBA{34, 211, 238, 255}  // Bright Cyan/Teal (Download)
	colorBlue       = color.RGBA{96, 165, 250, 255}  // Bright Blue (Upload)
	colorWhite      = color.RGBA{255, 255, 255, 255} // Pure White
	colorBrightGray = color.RGBA{203, 213, 225, 255} // Bright Gray (Text)
	colorGray       = color.RGBA{148, 163, 184, 255} // Muted Gray (Secondary)
	colorDarkGray   = color.RGBA{51, 65, 85, 255}    // Separator lines
	colorGaugeBg    = color.RGBA{30, 41, 59, 255}    // Gauge background
	colorGreen      = color.RGBA{74, 222, 128, 255}  // Bright Green (Good)
	colorYellow     = color.RGBA{250, 204, 21, 255}  // Bright Yellow (Warning)
	colorRed        = color.RGBA{248, 113, 113, 255} // Bright Red (Critical)
	colorAlertBg    = color.RGBA{127, 29, 29, 255}   // Alert bar background

	// Transparent fills for overlapping waves
	colorTealFill = color.RGBA{34, 211, 238, 60} // Transparent Teal
	colorBlueFill = color.RGBA{96, 165, 250, 60} // Transparent Blue

	// Notification theme colors
	colorNotifInfoBg     = color.RGBA{255, 255, 255, 255} // White background
	colorNotifInfoText   = color.RGBA{0, 0, 0, 255}       // Black text
	colorNotifWarnBg     = color.RGBA{250, 204, 21, 255}  // Yellow background
	colorNotifWarnText   = color.RGBA{0, 0, 0, 255}       // Black text
	colorNotifDangerBg   = color.RGBA{220, 38, 38, 255}   // Red background
	colorNotifDangerText = color.RGBA{255, 255, 255, 255} // White text

	// Login indicator
	colorLoginDot = color.RGBA{239, 68, 68, 255} // Bright red dot
)

// RenderStaticLayer renders the background elements that don't change often.
func RenderStaticLayer(img *image.RGBA, renderer *display.TextRenderer, state *State) {
	// Clear background
	display.FillImage(img, colorBackground)

	// Row 1: Header (0-52px)
	renderHeader(img, renderer, state)

	// Separator line
	display.DrawLine(img, 20, headerHeight-1, display.Width-20, colorDarkGray)

	// Row 3: Stats tiles (262-376px)
	renderStatsTiles(img, renderer, state)

	// Separator line (between speeds and stats)
	display.DrawLine(img, 20, headerHeight+speedsHeight-1, display.Width-20, colorDarkGray)
}

// RenderAndSend renders dynamic elements and sends the frame to the display.
func RenderAndSend(d *display.Display, img *image.RGBA, bgImg *image.RGBA, renderer *display.TextRenderer, state *State) {
	// 1. Copy static background (fast)
	display.CopyImage(img, bgImg)

	// 2. Draw dynamic elements (Waves + Speeds)
	renderSpeeds(img, renderer, state)

	// 3. Alert overlay (if any)
	if len(state.Alerts) > 0 {
		renderAlertBar(img, renderer, state.Alerts[0])
	}

	// 4. Login indicator (red dot in bottom-left if users logged in)
	if len(state.LoggedInUsers) > 0 {
		renderLoginIndicator(img)
	}

	// 5. Notification overlay (on top of everything)
	if state.ActiveNotification != nil {
		renderNotificationBox(img, renderer, state.ActiveNotification)
	}

	// 6. Store frame for screenshots
	state.SetLastFrame(img)

	// 7. Send to display
	if err := d.SendImage(img); err != nil {
		fmt.Printf("\nError sending image: %v\n", err)
	}
}

func renderHeader(img *image.RGBA, renderer *display.TextRenderer, state *State) {
	y := 36 // Baseline for 28px text in 52px header

	// Left: TrueNAS SCALE version
	renderer.DrawText(img, 20, y, "TrueNAS SCALE "+state.TrueNASVersion, colorWhite, 28, true)

	// Center: IP address
	renderer.DrawTextCentered(img, y, state.IPAddress, colorBrightGray, 28, false)

	// Right side: Interface + Link speed + Uptime
	rightText := state.Interface
	if state.LinkSpeed > 0 {
		rightText += " " + network.FormatLinkSpeed(state.LinkSpeed)
	}
	rightText += "  •  " + state.Uptime

	rightWidth := renderer.MeasureText(rightText, 26, false)
	renderer.DrawText(img, display.Width-rightWidth-20, y, rightText, colorGray, 26, false)
}

func renderSpeeds(img *image.RGBA, renderer *display.TextRenderer, state *State) {
	baseY := headerHeight
	height := speedsHeight
	centerY := baseY + height/2

	// Draw continuously scrolling waves (timestamp-based, no jumping)
	waveY := baseY + height - 10
	waveHeight := 100 // Height of the wave visualization

	// Get timestamped wave points from animator
	if state.WaveAnimator != nil && state.WaveAnimator.HasData() {
		rxPoints, txPoints := state.WaveAnimator.GetWavePoints(waveHeight)
		waveWidth, _ := state.WaveAnimator.GetConfig()

		// Convert wave.Point to display.WavePoint
		rxWavePoints := make([]display.WavePoint, len(rxPoints))
		txWavePoints := make([]display.WavePoint, len(txPoints))

		for i, p := range rxPoints {
			rxWavePoints[i] = display.WavePoint{X: p.X, Y: p.Y}
		}
		for i, p := range txPoints {
			txWavePoints[i] = display.WavePoint{X: p.X, Y: p.Y}
		}

		// Draw RX wave (Teal) - download
		if len(rxWavePoints) >= 2 {
			display.DrawTimestampedWave(img, waveY-waveHeight, waveHeight,
				waveWidth, rxWavePoints, colorTeal, colorTealFill)
		}

		// Draw TX wave (Blue) - upload (on top with transparency)
		if len(txWavePoints) >= 2 {
			display.DrawTimestampedWave(img, waveY-waveHeight, waveHeight,
				waveWidth, txWavePoints, colorBlue, colorBlueFill)
		}
	}

	// Draw Speed Text on top (uses averaged values for stability)
	rxSpeed := formatSpeedParts(state.DisplayRxSpeed)
	txSpeed := formatSpeedParts(state.DisplayTxSpeed)

	// Left side: Download
	drawSpeedDisplay(img, renderer, 80, centerY-20, "▼", rxSpeed, colorTeal)

	// Right side: Upload
	drawSpeedDisplay(img, renderer, display.Width/2+80, centerY-20, "▲", txSpeed, colorBlue)
}

type speedParts struct {
	Value string
	Unit  string
}

func formatSpeedParts(bytesPerSec float64) speedParts {
	bitsPerSec := bytesPerSec * 8
	const (
		Kbps = 1000
		Mbps = Kbps * 1000
		Gbps = Mbps * 1000
	)

	switch {
	case bitsPerSec >= Gbps:
		return speedParts{fmt.Sprintf("%.2f", bitsPerSec/Gbps), "Gb/s"}
	case bitsPerSec >= Mbps:
		return speedParts{fmt.Sprintf("%.1f", bitsPerSec/Mbps), "Mb/s"}
	case bitsPerSec >= Kbps:
		return speedParts{fmt.Sprintf("%.0f", bitsPerSec/Kbps), "Kb/s"}
	default:
		return speedParts{fmt.Sprintf("%.0f", bitsPerSec), "b/s"}
	}
}

func drawSpeedDisplay(img *image.RGBA, renderer *display.TextRenderer, x, centerY int, arrow string, speed speedParts, accentColor color.Color) {
	// Arrow
	renderer.DrawText(img, x, centerY-10, arrow, accentColor, 80, true)

	// Speed value (huge)
	valueX := x + 70
	renderer.DrawText(img, valueX, centerY+10, speed.Value, colorWhite, 120, true)

	// Unit
	valueWidth := renderer.MeasureText(speed.Value, 120, true)
	renderer.DrawText(img, valueX+valueWidth+12, centerY-20, speed.Unit, accentColor, 40, false)
}

func renderStatsTiles(img *image.RGBA, renderer *display.TextRenderer, state *State) {
	baseY := headerHeight + speedsHeight
	tileY := baseY + 5

	// Gauge parameters
	gaugeRadius := 35
	gaugeThickness := 9

	// Calculate tile positions
	positions := []int{120, 340, 560, 780}

	// Tile 1: CPU
	drawGaugeTile(img, renderer, positions[0], tileY, gaugeRadius, gaugeThickness,
		state.DisplayCPU, "CPU")

	// Tile 2: RAM
	memPercent := 0.0
	if state.Memory != nil {
		memPercent = state.Memory.UsedPercent
	}
	drawGaugeTile(img, renderer, positions[1], tileY, gaugeRadius, gaugeThickness,
		memPercent, "RAM")

	// Tiles 3-4: Pools
	for i := 0; i < 2 && i < len(state.Pools); i++ {
		pool := state.Pools[i]
		label := pool.Name
		if len(label) > 7 {
			label = label[:7]
		}
		drawGaugeTile(img, renderer, positions[2+i], tileY, gaugeRadius, gaugeThickness,
			pool.UsedPercent, label)
	}
}

func drawGaugeTile(img *image.RGBA, renderer *display.TextRenderer, cx, y, radius, thickness int, percent float64, label string) {
	cy := y + radius
	gaugeColor := getUsageColor(percent)

	display.DrawArcGaugeSimple(img, cx, cy, radius, thickness, percent, gaugeColor, colorGaugeBg)

	percentText := fmt.Sprintf("%.0f", percent)
	textWidth := renderer.MeasureText(percentText, 32, true)
	renderer.DrawText(img, cx-textWidth/2, cy+10, percentText, colorWhite, 32, true)

	labelWidth := renderer.MeasureText(label, 24, false)
	renderer.DrawText(img, cx-labelWidth/2, cy+radius+22, label, colorBrightGray, 24, false)
}

func getUsageColor(percent float64) color.Color {
	switch {
	case percent >= 90:
		return colorRed
	case percent >= 75:
		return colorYellow
	default:
		return colorGreen
	}
}

func renderAlertBar(img *image.RGBA, renderer *display.TextRenderer, alert string) {
	barY := headerHeight + 5
	barHeight := 36
	display.DrawFilledRect(img, 20, barY, display.Width-40, barHeight, colorAlertBg)
	alertText := "⚠ ALERT: " + alert
	renderer.DrawTextCentered(img, barY+26, alertText, colorWhite, 28, true)
}

// renderLoginIndicator draws a bright red dot in the bottom-left corner
// to indicate that users are logged into the shell.
func renderLoginIndicator(img *image.RGBA) {
	// Position: bottom-left corner with some padding
	cx := 20
	cy := display.Height - 20
	radius := 8

	display.DrawFilledCircle(img, cx, cy, radius, colorLoginDot)
}

// renderNotificationBox draws a centered notification box with rounded corners.
// The box supersedes all other content and is themed based on severity.
func renderNotificationBox(img *image.RGBA, renderer *display.TextRenderer, notif *Notification) {
	// Box dimensions
	boxWidth := 420
	boxHeight := 70
	borderRadius := 12

	// Center the box on screen
	boxX := (display.Width - boxWidth) / 2
	boxY := (display.Height - boxHeight) / 2

	// Select colors based on theme
	var bgColor, textColor color.Color
	switch notif.Theme {
	case ThemeWarning:
		bgColor = colorNotifWarnBg
		textColor = colorNotifWarnText
	case ThemeDanger:
		bgColor = colorNotifDangerBg
		textColor = colorNotifDangerText
	default: // ThemeInfo
		bgColor = colorNotifInfoBg
		textColor = colorNotifInfoText
	}

	// Draw rounded rectangle background
	display.DrawRoundedRect(img, boxX, boxY, boxWidth, boxHeight, borderRadius, bgColor)

	// Draw text centered in the box
	textY := boxY + boxHeight/2 + 10 // Baseline adjustment for vertical centering
	renderer.DrawTextCentered(img, textY, notif.Message, textColor, 28, true)
}
