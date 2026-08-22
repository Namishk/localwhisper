package main

import (
	"image"
	"image/color"
	"math"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"

	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"localwhisper/receiver/internal/overlay"
)

// The renderer draws a 52x44 unit glyph space scaled by S pixels per unit.
// Motions match the approved mocks: R1 recording, T1 transcribing,
// C1 copied, F3 failed, D1 disconnected.
const (
	S           = 2.0
	windowTitle = "LocalWhisperOverlay"
	cycle       = 900 * time.Millisecond // shared loop for recording/transcribing
)

type rgb struct{ r, g, b uint8 }

var (
	pinkRGB  = rgb{255, 107, 107}
	blueRGB  = rgb{116, 192, 252}
	greenRGB = rgb{105, 219, 124}
	amberRGB = rgb{255, 212, 59}

	pillBG   = color.NRGBA{24, 24, 27, 245}
	pillBD   = color.NRGBA{255, 255, 255, 26}
	slashCut = color.NRGBA{24, 24, 27, 255} // underlay matching the pill background
)

func shade(c rgb, a float64) color.NRGBA {
	return color.NRGBA{c.r, c.g, c.b, uint8(math.Round(math.Min(1, math.Max(0, a)) * 255))}
}

func clamp01(x float64) float64 { return math.Min(1, math.Max(0, x)) }

func sincePhase(age, delay, dur time.Duration) float64 {
	if age < delay {
		return 0
	}
	return clamp01(float64(age-delay) / float64(dur))
}

func easeOutBack(p float64) float64 {
	const overshoot = 1.70158
	p -= 1
	return math.Max(0.01, 1+(overshoot+1)*p*p*p+overshoot*p*p)
}

type ptF struct{ x, y float32 }

func lerpPt(a, b ptF, t float64) ptF {
	return ptF{a.x + (b.x-a.x)*float32(t), a.y + (b.y-a.y)*float32(t)}
}

func arcPoints(cx, cy, radius, start, end float64, steps int) []ptF {
	pts := make([]ptF, 0, steps+1)
	for i := 0; i <= steps; i++ {
		a := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, ptF{float32(cx + radius*math.Cos(a)), float32(cy + radius*math.Sin(a))})
	}
	return pts
}

func polylineLength(pts []ptF) float64 {
	total := 0.0
	for i := 1; i < len(pts); i++ {
		total += math.Hypot(float64(pts[i].x-pts[i-1].x), float64(pts[i].y-pts[i-1].y))
	}
	return total
}

// partialPolyline truncates a point run at fraction t of its total length.
func partialPolyline(pts []ptF, t float64) []ptF {
	if t >= 1 {
		return pts
	}
	target := polylineLength(pts) * t
	out := []ptF{pts[0]}
	walked := 0.0
	for i := 1; i < len(pts); i++ {
		seg := math.Hypot(float64(pts[i].x-pts[i-1].x), float64(pts[i].y-pts[i-1].y))
		if walked+seg >= target {
			out = append(out, lerpPt(pts[i-1], pts[i], (target-walked)/seg))
			return out
		}
		walked += seg
		out = append(out, pts[i])
	}
	return out
}

func strokePts(dst *ebiten.Image, pts []ptF, width float32, clr color.Color) {
	for i := 1; i < len(pts); i++ {
		vector.StrokeLine(dst, pts[i-1].x, pts[i-1].y, pts[i].x, pts[i].y, width, clr, true)
	}
}

// Game renders the status pill inside an undecorated transparent window.
type Game struct {
	store *overlay.Store
	texts map[string]*ebiten.Image

	tick       int
	lastLayout string
	w, h       int
}

func NewGame(store *overlay.Store) *Game {
	return &Game{store: store, texts: map[string]*ebiten.Image{}, w: 160, h: 124}
}

// layoutFor sizes the window to the pill contents.
func layoutFor(state string) (w, h int, glyphX, glyphY float32, text string) {
	text = overlay.Label(state)
	if text == "" {
		return int((52 + 28) * S), int((44 + 18) * S), 14 * S, 9 * S, ""
	}
	totalUnits := 52 + 10 + len(text)*7 + 36
	return int(totalUnits * S), int((44 + 28) * S), 18 * S, 14 * S, text
}

func (g *Game) Update() error {
	g.tick++
	state, _ := g.store.Current()
	w, h, _, _, _ := layoutFor(state)
	sig := strconv.Itoa(w) + "x" + strconv.Itoa(h)
	if sig != g.lastLayout {
		g.lastLayout, g.w, g.h = sig, w, h
		ebiten.SetWindowSize(w, h)
		g.reposition()
	}
	if g.tick%30 == 0 {
		g.reposition()
	}
	return nil
}

func (g *Game) reposition() {
	sw, sh := ebiten.ScreenSizeInFullscreen()
	ebiten.SetWindowPosition((sw-g.w)/2, sh-g.h-48)
}

func (g *Game) Layout(int, int) (int, int) { return g.w, g.h }

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.NRGBA{})
	state, age := g.store.Current()
	if state == "" {
		return
	}
	w, h, gx, gy, text := layoutFor(state)
	drawPill(screen, float32(w), float32(h))
	cx, cy := gx+26*S, gy+22*S
	switch state {
	case "recording":
		drawRecording(screen, cx, cy, age)
	case "transcribing":
		drawTranscribing(screen, cx, cy, age)
	case "copied":
		drawCopied(screen, cx, cy, age)
	case "failed":
		drawFailed(screen, cx, cy, age)
	case "disconnected":
		drawDisconnected(screen, cx, gy, age)
	}
	if text != "" {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(S, S)
		op.GeoM.Translate(float64(gx+52*S+10*S), float64((float32(h)-13*S)/2))
		screen.DrawImage(g.textImage(text), op)
	}
}

func drawPill(dst *ebiten.Image, w, h float32) {
	r := h / 2
	vector.DrawFilledCircle(dst, r, r, r, pillBG, true)
	vector.DrawFilledCircle(dst, w-r, r, r, pillBG, true)
	vector.DrawFilledRect(dst, r, 0, w-2*r, h, pillBG, true)
	vector.StrokeCircle(dst, r, r, r-1, 1, pillBD, true)
	vector.StrokeCircle(dst, w-r, r, r-1, 1, pillBD, true)
	vector.StrokeLine(dst, r, 0.5, w-r, 0.5, 1, pillBD, true)
	vector.StrokeLine(dst, r, h-0.5, w-r, h-0.5, 1, pillBD, true)
}

// R1 recording: two expanding rings plus breathing mic body.
func drawRecording(dst *ebiten.Image, cx, cy float32, age time.Duration) {
	phase := float64(age%cycle) / float64(cycle)
	for i := range 2 {
		travel := math.Mod(phase+float64(i)*0.5, 1)
		radius := (13 + travel*9) * S
		alpha := 0.5 * math.Pow(1-travel, 1.6)
		vector.StrokeCircle(dst, cx, cy-2*S, float32(radius), 2*S, shade(pinkRGB, alpha), true)
	}
	breath := 1 + 0.045*math.Sin(2*math.Pi*phase)
	drawMic(dst, cx, cy, breath)
}

func drawMic(dst *ebiten.Image, cx, cy float32, breath float64) {
	at := func(x, y float64) ptF {
		x = 26 + (x-26)*breath
		y = 22 + (y-22)*breath
		return ptF{cx + float32(x-26)*S, cy + float32(y-22)*S}
	}
	body := shade(pinkRGB, 0.95)
	top, bottom := at(26, 14.5), at(26, 17.5)
	radius := float32(4.5 * breath * S)
	vector.DrawFilledCircle(dst, top.x, top.y, radius, body, true)
	vector.DrawFilledCircle(dst, bottom.x, bottom.y, radius, body, true)
	leftX, rightX := at(21.5, 16).x, at(30.5, 16).x
	vector.DrawFilledRect(dst, leftX, top.y, rightX-leftX, bottom.y-top.y, body, true)

	cradle := arcPoints(26, 20, 8, -0.55, math.Pi+0.55, 24)
	mapped := make([]ptF, len(cradle))
	for i, p := range cradle {
		mapped[i] = at(float64(p.x), float64(p.y))
	}
	strokePts(dst, mapped, 2.4*S, body)
	strokePts(dst, []ptF{at(26, 28), at(26, 31)}, 2.4*S, body)
	strokePts(dst, []ptF{at(21, 31), at(31, 31)}, 2.4*S, body)
}

// T1 transcribing: four bars with staggered sine levels.
func drawTranscribing(dst *ebiten.Image, cx, cy float32, age time.Duration) {
	phase := float64(age%cycle) / float64(cycle)
	dx := [4]float64{-10.5, -3.5, 3.5, 10.5}
	offsets := [4]float64{5.1, 1.7, 3.4, 0.4}
	amps := [4]float64{0.62, 1.0, 0.92, 0.58}
	clr := shade(blueRGB, 0.95)
	for i := range dx {
		level := math.Pow(0.5+0.5*math.Sin(2*math.Pi*phase+offsets[i]), 1.6)
		half := float32((3.5 + 6*amps[i]*level) * S)
		x := cx + float32(dx[i])*S
		vector.StrokeLine(dst, x, cy-half, x, cy+half, 2.4*S, clr, true)
	}
}

var checkPath = []ptF{{20, 22.5}, {24.5, 27}, {32.5, 17}}

func checkPathAround(cx, cy float32) []ptF {
	out := make([]ptF, len(checkPath))
	for i, p := range checkPath {
		out[i] = ptF{cx + (p.x-26)*S, cy + (p.y-22)*S}
	}
	return out
}

// C1 copied: badge pops in while the check strokes itself on.
func drawCopied(dst *ebiten.Image, cx, cy float32, age time.Duration) {
	pop := easeOutBack(sincePhase(age, 0, 280*time.Millisecond))
	fade := clamp01(pop * 3)
	ringR := float32(14 * S * pop)
	if fill := shade(greenRGB, 0.14*fade); fill.A > 0 {
		vector.DrawFilledCircle(dst, cx, cy, ringR, fill, true)
	}
	if line := shade(greenRGB, 0.92*fade); line.A > 0 && ringR > 1 {
		vector.StrokeCircle(dst, cx, cy, ringR, 2*S, line, true)
	}
	checkP := sincePhase(age, 120*time.Millisecond, 350*time.Millisecond)
	if checkP > 0 {
		strokePts(dst, partialPolyline(checkPathAround(cx, cy), checkP), 2.8*S, shade(greenRGB, 0.95))
	}
}

// F3 failed: ring holds still, exclamation draws itself, dot pops.
func drawFailed(dst *ebiten.Image, cx, cy float32, age time.Duration) {
	vector.DrawFilledCircle(dst, cx, cy, 14*S, shade(amberRGB, 0.14), true)
	vector.StrokeCircle(dst, cx, cy, 14*S, 2*S, shade(amberRGB, 0.92), true)
	lineP := sincePhase(age, 60*time.Millisecond, 180*time.Millisecond)
	if lineP > 0 {
		end := lerpPt(ptF{cx, cy - 7*S}, ptF{cx, cy + 2*S}, lineP)
		vector.StrokeLine(dst, cx, cy-7*S, end.x, end.y, 2.4*S, shade(amberRGB, 0.95), true)
	}
	dotP := sincePhase(age, 220*time.Millisecond, 150*time.Millisecond)
	if dotP > 0 {
		vector.DrawFilledCircle(dst, cx, cy+6.5*S, float32(1.7*easeOutBack(dotP))*S, shade(amberRGB, 0.95), true)
	}
}

// D1 disconnected: phone outline pops, slash cuts across it.
func drawDisconnected(dst *ebiten.Image, cx, glyphY float32, age time.Duration) {
	cy := glyphY + 22*S
	phoneCX := cx - 3*S // phone body is offset left of the glyph center
	pop := easeOutBack(sincePhase(age, 0, 280*time.Millisecond))
	if pop > 0 {
		drawPhoneOutline(dst, phoneCX, cy, pop)
	}
	slashP := sincePhase(age, 120*time.Millisecond, 300*time.Millisecond)
	if slashP > 0 {
		a, b := ptF{cx - 11*S, cy - 11*S}, ptF{cx + 9*S, cy + 9*S}
		end := lerpPt(a, b, slashP)
		vector.StrokeLine(dst, a.x, a.y, end.x, end.y, 6.5*S, slashCut, true)
		vector.StrokeLine(dst, a.x, a.y, end.x, end.y, 3.5*S, shade(amberRGB, 0.95), true)
	}
}

func drawPhoneOutline(dst *ebiten.Image, cx, cy float32, scale float64) {
	clr := shade(amberRGB, 0.95)
	halfW, halfH := float32(5*scale)*S, float32(11*scale)*S
	x0, x1 := cx-halfW, cx+halfW
	y0, y1 := cy-halfH, cy+halfH
	r := float32(3*scale) * S

	corners := [][]ptF{
		arcPoints(float64(x1-r), float64(y0+r), float64(r), -math.Pi/2, 0, 6),
		arcPoints(float64(x1-r), float64(y1-r), float64(r), 0, math.Pi/2, 6),
		arcPoints(float64(x0+r), float64(y1-r), float64(r), math.Pi/2, math.Pi, 6),
		arcPoints(float64(x0+r), float64(y0+r), float64(r), math.Pi, 3*math.Pi/2, 6),
	}
	for i := range corners {
		next := corners[(i+1)%len(corners)]
		pts := append(append([]ptF{}, corners[i]...), next[0])
		strokePts(dst, pts, 2.4*S, clr)
	}
	strokePts(dst, []ptF{{x1 - r, y0}, {x0 + r, y0}}, 2.4*S, clr)
	strokePts(dst, []ptF{{x1 - r, y1}, {x0 + r, y1}}, 2.4*S, clr)
}

func (g *Game) textImage(text string) *ebiten.Image {
	if img, ok := g.texts[text]; ok {
		return img
	}
	face := basicfont.Face7x13
	width := face.Advance * len(text)
	rgba := image.NewNRGBA(image.Rect(0, 0, width, face.Height))
	d := &font.Drawer{
		Dst:  rgba,
		Src:  image.White,
		Face: face,
		Dot:  fixed.P(0, face.Ascent),
	}
	d.DrawString(text)
	img := ebiten.NewImageFromImage(rgba)
	g.texts[text] = img
	return img
}
