package dashboard

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"go-wtr-display/display"
	"go-wtr-display/network"
)

var (
	colorBackground = color.RGBA{8, 15, 24, 255}
	colorWhite      = color.RGBA{241, 247, 252, 255}
	colorGray       = color.RGBA{165, 185, 202, 255}
	colorMuted      = color.RGBA{75, 95, 114, 255}
	colorLine       = color.RGBA{30, 48, 65, 255}
	colorTeal       = color.RGBA{35, 218, 221, 255}
	colorBlue       = color.RGBA{104, 166, 255, 255}
	colorGreen      = color.RGBA{80, 220, 155, 255}
	colorYellow     = color.RGBA{255, 199, 90, 255}
	colorRed        = color.RGBA{255, 111, 117, 255}
)

// FrameRenderer caches the entire non-animated view. The key uses displayed
// precision, so hidden metrics and insignificant changes do not trigger work.
type FrameRenderer struct {
	text        *display.TextRenderer
	base, frame *image.RGBA
	key         string
}

func NewFrameRenderer() *FrameRenderer {
	return &FrameRenderer{text: display.NewTextRenderer(), base: display.NewDisplayImage(), frame: display.NewDisplayImage()}
}

func (r *FrameRenderer) Frame(s *State, p Page, o Options, now time.Time, force bool) (*image.RGBA, bool) {
	key := visualKey(s, p, o)
	waveVisible := p.View == ViewNetwork && o.WaveStyle != "off" && s.NetworkAvailable && s.WaveAnimator != nil && !CurrentHealth(s).Critical && s.GetActiveNotification() == nil
	moving := waveVisible && s.WaveAnimator.HasMotion(now)
	if key == r.key && !moving && !force {
		return nil, false
	}
	if key != r.key || force {
		renderBase(r.base, r.text, s, p, o)
		r.key = key
	}
	display.CopyImage(r.frame, r.base)
	if waveVisible {
		renderWave(r.frame, s, o, now)
	}
	return r.frame, true
}

func visualKey(s *State, p Page, o Options) string {
	h := CurrentHealth(s)
	key := fmt.Sprintf("%v|%v|%v|%t|%s", p, h, o.Views, o.Fixed, o.Pool)
	if n := s.GetActiveNotification(); n != nil {
		key += fmt.Sprintf("|%s|%s", n.Theme, n.Message)
	}
	if h.Critical {
		return key
	}
	switch p.View {
	case ViewNetwork:
		rx := formatSpeedParts(s.DisplayRxSpeed, &s.LastRxUnit)
		tx := formatSpeedParts(s.DisplayTxSpeed, &s.LastTxUnit)
		key += fmt.Sprintf("|%v|%v|%t|%d|%s|%s|%s|%v", rx, tx, s.NetworkAvailable, s.LinkSpeed, s.Interface, s.IPAddress, o.WaveStyle, o.WaveHistory)
	case ViewStorage:
		pools := visiblePools(s.Pools)
		if p.Pool >= 0 && p.Pool < len(pools) {
			key += fmt.Sprintf("|%v|%s", pools[p.Pool], poolHealth(s, pools[p.Pool].Name))
		}
		key += fmt.Sprintf("|%d", len(pools))
	case ViewSystem:
		key += fmt.Sprintf("|%.0f|%t|%s|%s|%s", s.DisplayCPU, s.CPUAvailable, s.TrueNASVersion, s.IPAddress, s.Uptime)
		if s.Memory != nil {
			key += fmt.Sprintf("|%.1f|%.0f", s.Memory.AvailableGB, s.Memory.TotalGB)
		}
		if s.ARC != nil {
			key += fmt.Sprintf("|%.1f", s.ARC.SizeGB)
		}
		if s.Temperatures.CPU != nil {
			key += fmt.Sprintf("|%.0f", *s.Temperatures.CPU)
		}
	case ViewDrives:
		for _, slot := range s.Disks {
			temp, ok := s.Temperatures.Disks[slot.Device]
			key += fmt.Sprintf("|%s:%d:%t:%.0f:%t", slot.Kind, slot.Bay, slot.Present, temp, ok)
		}
	}
	if len(s.LoggedInUsers) > 0 {
		key += "|shell"
	}
	return key
}

func renderBase(img *image.RGBA, r *display.TextRenderer, s *State, p Page, o Options) {
	display.FillImage(img, colorBackground)
	h := CurrentHealth(s)
	accent := colorGreen
	if h.Warning {
		accent = colorYellow
	}
	if h.Critical {
		accent = colorRed
	}
	title := strings.ToUpper(string(p.View))
	if h.Critical {
		title = "ATTENTION"
	}
	r.DrawText(img, 28, 37, title, colorWhite, 30, true)
	label := h.Title
	if !h.Critical && h.Detail != "" {
		label += " / " + h.Detail
	}
	label = ellipsize(r, label, 570, 24, true)
	r.DrawText(img, 932-r.MeasureText(label, 24, true), 36, label, accent, 24, true)
	display.DrawLine(img, 28, 53, 932, colorLine)
	renderNavigation(img, r, p, o)
	if h.Critical {
		renderMessage(img, r, h.Title, h.Detail, colorRed)
		return
	}
	if n := s.GetActiveNotification(); n != nil {
		c := colorTeal
		if n.Theme == ThemeWarning {
			c = colorYellow
		}
		renderMessage(img, r, "NOTICE", n.Message, c)
		return
	}
	switch p.View {
	case ViewStorage:
		renderStorage(img, r, s, p, o)
	case ViewSystem:
		renderSystem(img, r, s)
	case ViewDrives:
		renderDrives(img, r, s)
	default:
		renderNetwork(img, r, s, o)
	}
	if len(s.LoggedInUsers) > 0 {
		// A labelled session indicator avoids confusing shell access with a fault.
		r.DrawText(img, 812, 363, "SSH", colorGray, 22, true)
	}
}

func renderNavigation(img *image.RGBA, r *display.TextRenderer, p Page, o Options) {
	display.DrawLine(img, 28, 337, 932, colorLine)
	x := 28
	for _, v := range o.Views {
		label := strings.ToUpper(string(v))
		c := colorMuted
		if v == p.View {
			c = colorWhite
			display.DrawFilledRect(img, x, 335, r.MeasureText(label, 22, true), 3, colorTeal)
		}
		r.DrawText(img, x, 365, label, c, 22, true)
		x += r.MeasureText(label, 22, true) + 32
	}
	mode := "AUTO"
	if o.Fixed {
		mode = "FIXED"
	}
	r.DrawText(img, 932-r.MeasureText(mode, 22, true), 365, mode, colorGray, 22, true)
}

func renderNetwork(img *image.RGBA, r *display.TextRenderer, s *State, o Options) {
	rx := formatSpeedParts(s.DisplayRxSpeed, &s.LastRxUnit)
	tx := formatSpeedParts(s.DisplayTxSpeed, &s.LastTxUnit)
	for i, item := range []struct {
		label string
		speed speedParts
		c     color.Color
	}{{"DOWNLOAD", rx, colorTeal}, {"UPLOAD", tx, colorBlue}} {
		x := 32 + i*480
		drawArrow(img, x+10, 90, i == 0, item.c)
		r.DrawText(img, x+36, 99, item.label, item.c, 29, true)
		value, unit := item.speed.Value, item.speed.Unit
		if !s.NetworkAvailable {
			value = "--"
			unit = "unavailable"
		}
		drawFit(img, r, x, 201, 330, value, colorWhite, 108, 72, true)
		r.DrawText(img, x+400-r.MeasureText(unit, 29, false), 200, unit, item.c, 29, false)
	}
	meta := s.Interface
	if s.LinkSpeed > 0 {
		meta += " / " + network.FormatLinkSpeed(s.LinkSpeed) + " link"
	}
	r.DrawText(img, 32, 231, ellipsize(r, meta, 450, 23, false), colorGray, 23, false)
	scale := "adaptive"
	if o.WaveScale == "link" && s.LinkSpeed > 0 {
		scale = "link scale"
	}
	history := fmt.Sprintf("%gs / %s", o.WaveHistory.Seconds(), scale)
	if o.WaveStyle == "off" {
		history = s.IPAddress
	}
	r.DrawText(img, 928-r.MeasureText(history, 23, false), 231, history, colorGray, 23, false)
	display.DrawLine(img, 0, 320, display.Width, colorLine)
}

func renderWave(img *image.RGBA, s *State, o Options, now time.Time) {
	ceiling := 0.0
	if o.WaveScale == "link" && s.LinkSpeed > 0 {
		ceiling = float64(s.LinkSpeed) * 1e6 / 8
	}
	rx, tx := s.WaveAnimator.PointsAt(now, ceiling)
	var rxFill, txFill color.Color
	if o.WaveStyle == "filled" {
		rxFill = color.NRGBA{35, 218, 221, 65}
		txFill = color.NRGBA{104, 166, 255, 65}
	}
	display.DrawTimestampedWave(img, 242, 78, display.Width, rx, colorTeal, rxFill)
	display.DrawTimestampedWave(img, 242, 78, display.Width, tx, colorBlue, txFill)
}

func renderStorage(img *image.RGBA, r *display.TextRenderer, s *State, p Page, o Options) {
	pools := visiblePools(s.Pools)
	if p.Pool < 0 || p.Pool >= len(pools) || s.PoolReadFailed {
		detail := "No pool capacity readings"
		if o.Pool != "" {
			detail = "Pool not found: " + o.Pool
		}
		renderMessage(img, r, "STORAGE UNAVAILABLE", detail, colorYellow)
		return
	}
	pool := pools[p.Pool]
	r.DrawText(img, 32, 105, ellipsize(r, pool.Name, 650, 42, true), colorWhite, 42, true)
	health := poolHealth(s, pool.Name)
	c := colorGray
	if health == "ONLINE" {
		c = colorGreen
	}
	if health != "ONLINE" && health != "UNKNOWN" {
		c = colorRed
	}
	r.DrawText(img, 928-r.MeasureText(health, 27, true), 100, health, c, 27, true)
	drawFit(img, r, 28, 250, 430, fmt.Sprintf("%.0f%%", pool.UsedPercent), colorWhite, 140, 100, true)
	r.DrawText(img, 520, 158, "AVAILABLE", colorGray, 29, true)
	drawFit(img, r, 515, 242, 405, pool.Free, colorTeal, 86, 58, true)
	display.DrawFilledRect(img, 32, 278, 896, 14, colorLine)
	used := int(896 * math.Max(0, math.Min(100, pool.UsedPercent)) / 100)
	display.DrawFilledRect(img, 32, 278, used, 14, colorTeal)
	summary := pool.Used + " used / " + pool.Size + " usable"
	r.DrawText(img, 32, 325, ellipsize(r, summary, 740, 27, false), colorGray, 27, false)
	count := fmt.Sprintf("%d / %d", p.Pool+1, len(pools))
	r.DrawText(img, 928-r.MeasureText(count, 23, false), 325, count, colorGray, 23, false)
}

func poolHealth(s *State, name string) string {
	if s.ZFSStatus != nil && !s.ZFSReadFailed {
		if h := s.ZFSStatus.PoolStates[name]; h != "" {
			return h
		}
	}
	return "UNKNOWN"
}

func renderSystem(img *image.RGBA, r *display.TextRenderer, s *State) {
	labels := []string{"CPU", "MEMORY", "ZFS CACHE"}
	for i, label := range labels {
		r.DrawText(img, 32+i*316, 104, label, colorGray, 30, true)
	}
	cpu := "--"
	if s.CPUAvailable {
		cpu = fmt.Sprintf("%.0f%%", s.DisplayCPU)
	}
	drawFit(img, r, 26, 219, 275, cpu, colorWhite, 100, 68, true)
	temperature := "TEMP UNAVAILABLE"
	if s.Temperatures.CPU != nil {
		temperature = fmt.Sprintf("%.0f°C", *s.Temperatures.CPU)
	}
	r.DrawText(img, 32, 270, temperature, colorTeal, 27, true)
	memory, arc := "--", "--"
	if s.Memory != nil {
		memory = fmt.Sprintf("%.1f", s.Memory.AvailableGB)
	}
	if s.ARC != nil {
		arc = fmt.Sprintf("%.1f", s.ARC.SizeGB)
	}
	drawFit(img, r, 342, 214, 275, memory, colorWhite, 84, 60, true)
	drawFit(img, r, 658, 214, 275, arc, colorWhite, 84, 60, true)
	r.DrawText(img, 348, 266, "GiB AVAILABLE", colorTeal, 26, true)
	r.DrawText(img, 664, 266, "GiB IN ARC", colorBlue, 26, true)
	total := "RAM UNKNOWN"
	if s.Memory != nil {
		total = fmt.Sprintf("%.0f GiB RAM", s.Memory.TotalGB)
	}
	detail := fmt.Sprintf("%s / %s / %s / up %s", s.TrueNASVersion, s.IPAddress, total, s.Uptime)
	r.DrawText(img, 32, 323, ellipsize(r, detail, 896, 24, false), colorGray, 24, false)
}

func renderDrives(img *image.RGBA, r *display.TextRenderer, s *State) {
	if s.Disks == nil {
		renderMessage(img, r, "DRIVES UNAVAILABLE", "Drive inventory could not be read", colorYellow)
		return
	}
	r.DrawText(img, 32, 92, "SATA", colorGray, 27, true)
	present := 0
	for _, slot := range s.Disks {
		if slot.Present {
			present++
		}
	}
	summary := fmt.Sprintf("%d / 11 OCCUPIED", present)
	r.DrawText(img, 928-r.MeasureText(summary, 25, true), 92, summary, colorGray, 25, true)
	for i := 0; i < 6; i++ {
		drawBay(img, r, s, findSlot(s.Disks, "sata", i+1), 32+i*150, 105, 138, 106)
	}
	r.DrawText(img, 32, 246, "NVMe", colorGray, 27, true)
	for i := 0; i < 5; i++ {
		drawBay(img, r, s, findSlot(s.Disks, "nvme", i+1), 32+i*180, 257, 168, 65)
	}
}

func findSlot(slots []network.DiskSlot, kind string, bay int) network.DiskSlot {
	for _, s := range slots {
		if s.Kind == kind && s.Bay == bay {
			return s
		}
	}
	return network.DiskSlot{Kind: kind, Bay: bay}
}

func drawBay(img *image.RGBA, r *display.TextRenderer, s *State, slot network.DiskSlot, x, y, w, h int) {
	c := colorLine
	if slot.Present {
		c = colorTeal
	}
	display.DrawFilledRect(img, x, y, w, h, c)
	display.DrawFilledRect(img, x+2, y+2, w-4, h-4, colorBackground)
	label := fmt.Sprint(slot.Bay)
	if slot.Kind == "nvme" {
		label = "N" + label
	}
	fg := colorMuted
	if slot.Present {
		fg = colorWhite
	}
	temp := "EMPTY"
	if slot.Present {
		temp = "--"
		if v, ok := s.Temperatures.Disks[slot.Device]; ok {
			temp = fmt.Sprintf("%.0f°C", v)
		}
	}
	if slot.Kind == "sata" {
		r.DrawText(img, x+(w-r.MeasureText(label, 44, true))/2, y+49, label, fg, 44, true)
		r.DrawText(img, x+(w-r.MeasureText(temp, 26, false))/2, y+85, temp, fg, 26, false)
	} else {
		r.DrawText(img, x+12, y+42, label, fg, 29, true)
		r.DrawText(img, x+w-r.MeasureText(temp, 24, false)-12, y+42, temp, fg, 24, false)
	}
}

func renderMessage(img *image.RGBA, r *display.TextRenderer, title, detail string, c color.Color) {
	display.DrawFilledRect(img, 32, 92, 5, 194, c)
	drawFit(img, r, 64, 161, 832, title, c, 58, 38, true)
	// Two measured lines preserve long notifications without shrinking the UI.
	words := strings.Fields(detail)
	line := ""
	lines := []string{}
	for _, word := range words {
		next := strings.TrimSpace(line + " " + word)
		if r.MeasureText(next, 34, false) > 830 && line != "" {
			lines = append(lines, line)
			line = word
		} else {
			line = next
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	for i, line := range lines {
		if i == 2 {
			break
		}
		if i == 1 && len(lines) > 2 {
			line += " ..."
		}
		r.DrawText(img, 64, 222+i*47, ellipsize(r, line, 830, 34, false), colorWhite, 34, false)
	}
}

func drawArrow(img *image.RGBA, x, y int, down bool, c color.Color) {
	// Native geometry keeps glyph availability independent of the font.
	for row := 0; row < 14; row++ {
		w := 14 - row
		if !down {
			w = row + 1
		}
		display.DrawFilledRect(img, x-w, y+row, w*2, 1, c)
	}
}

func drawFit(img *image.RGBA, r *display.TextRenderer, x, y, width int, text string, c color.Color, size, min float64, bold bool) {
	for size > min && r.MeasureText(text, size, bold) > width {
		size -= 2
	}
	r.DrawText(img, x, y, ellipsize(r, text, width, size, bold), c, size, bold)
}

func ellipsize(r *display.TextRenderer, text string, width int, size float64, bold bool) string {
	if r.MeasureText(text, size, bold) <= width {
		return text
	}
	runes := []rune(text)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		s := string(runes) + "..."
		if r.MeasureText(s, size, bold) <= width {
			return s
		}
	}
	return ""
}
