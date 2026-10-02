package tui

import "math"

// Oklab and OKHSL <-> sRGB conversion, ported from pi-tui (oklab.js and the
// OKLCH helpers in colors.js) for Pi's system theme. Oklab and OKHSL are
// Björn Ottosson's colour spaces; this follows his reference implementation
// (https://bottosson.github.io/posts/colorpicker/), Copyright (c) 2021 Björn
// Ottosson, used under the MIT license.

type vec3 [3]float64

func mul3(m [3]vec3, v vec3) vec3 {
	return vec3{
		m[0][0]*v[0] + m[0][1]*v[1] + m[0][2]*v[2],
		m[1][0]*v[0] + m[1][1]*v[1] + m[1][2]*v[2],
		m[2][0]*v[0] + m[2][1]*v[1] + m[2][2]*v[2],
	}
}

var (
	linearSrgbToLMS = [3]vec3{
		{0.4122214694707629, 0.5363325372617349, 0.0514459932675022},
		{0.2119034958178251, 0.6806995506452344, 0.1073969535369405},
		{0.0883024591900564, 0.2817188391361215, 0.6299787016738222},
	}
	lmsToLab = [3]vec3{
		{0.210454268309314, 0.793617774702305, -0.0040720430116193},
		{1.9779985324311684, -2.42859224204858, 0.450593709617411},
		{0.0259040424655478, 0.7827717124575296, -0.8086757549230774},
	}
	labToLMS = [3]vec3{
		{1, 0.3963377773761749, 0.2158037573099136},
		{1, -0.1055613458156586, -0.0638541728258133},
		{1, -0.0894841775298119, -1.2914855480194092},
	}
	lmsToLinearSrgb = [3]vec3{
		{4.0767416360759583, -3.3077115392580629, 0.2309699031821043},
		{-1.2684379732850315, 2.6097573492876882, -0.341319376002657},
		{-0.0041960761386756, -0.7034186179359362, 1.7076146940746117},
	}
	// Per sRGB channel: the (a, b) half-plane where it clips first, and the
	// polynomial approximating the maximum saturation there.
	saturationFit = [3]struct {
		plane vec3
		k     [5]float64
	}{
		{vec3{-1.8817031, -0.80936501}, [5]float64{1.19086277, 1.76576728, 0.59662641, 0.75515197, 0.56771245}},
		{vec3{1.8144408, -1.19445267}, [5]float64{0.73956515, -0.45954404, 0.08285427, 0.12541073, -0.14503204}},
		{vec3{0.13110758, 1.81333971}, [5]float64{1.35733652, -0.00915799, -1.1513021, -0.50559606, 0.00692167}},
	}
)

const (
	okK1 = 0.206
	okK2 = 0.03
	okK3 = (1 + okK1) / (1 + okK2)
)

// oklabToOkhslLightness converts Oklab lightness to OKHSL lightness.
func oklabToOkhslLightness(x float64) float64 {
	return 0.5 * (okK3*x - okK1 + math.Sqrt((okK3*x-okK1)*(okK3*x-okK1)+4*okK2*okK3*x))
}

func okhslToOklabLightness(x float64) float64 { return (x*x + okK1*x) / (okK3 * (x + okK2)) }

func linearToSrgb(v float64) float64 {
	if v > 0.0031308 {
		return 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return 12.92 * v
}

func srgbToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func oklabToLinearSrgb(lab vec3) vec3 {
	lms := mul3(labToLMS, lab)
	for i := range lms {
		lms[i] = lms[i] * lms[i] * lms[i]
	}
	return mul3(lmsToLinearSrgb, lms)
}

func rgbToOklab(c rgb) vec3 {
	lin := vec3{srgbToLinear(float64(c.r) / 255), srgbToLinear(float64(c.g) / 255), srgbToLinear(float64(c.b) / 255)}
	lms := mul3(linearSrgbToLMS, lin)
	for i := range lms {
		lms[i] = math.Cbrt(lms[i])
	}
	return mul3(lmsToLab, lms)
}

// jsRound is JavaScript's Math.round (halves round up).
func jsRound(x float64) float64 { return math.Floor(x + 0.5) }

// linearSrgbToRgb clips out-of-gamut channels and rounds to 0-255.
func linearSrgbToRgb(lin vec3) rgb {
	ch := func(v float64) uint8 {
		return uint8(jsRound(math.Min(1, math.Max(0, linearToSrgb(v))) * 255))
	}
	return rgb{ch(lin[0]), ch(lin[1]), ch(lin[2])}
}

func lmsSlopes(a, b float64) vec3 {
	return vec3{labToLMS[0][1]*a + labToLMS[0][2]*b, labToLMS[1][1]*a + labToLMS[1][2]*b, labToLMS[2][1]*a + labToLMS[2][2]*b}
}

func maxSaturation(a, b float64) float64 {
	channel := 2
	for i := 0; i < 2; i++ {
		if saturationFit[i].plane[0]*a+saturationFit[i].plane[1]*b > 1 {
			channel = i
			break
		}
	}
	k := saturationFit[channel].k
	w := lmsToLinearSrgb[channel]
	s := k[0] + k[1]*a + k[2]*b + k[3]*a*a + k[4]*a*b
	slopes := lmsSlopes(a, b)
	var f, f1, f2 float64
	for i := 0; i < 3; i++ {
		base := 1 + s*slopes[i]
		f += w[i] * base * base * base
		f1 += w[i] * 3 * slopes[i] * base * base
		f2 += w[i] * 6 * slopes[i] * slopes[i] * base
	}
	return s - (f*f1)/(f1*f1-0.5*f*f2)
}

func okCusp(a, b float64) (float64, float64) {
	s := maxSaturation(a, b)
	lin := oklabToLinearSrgb(vec3{1, s * a, s * b})
	l := math.Cbrt(1 / math.Max(lin[0], math.Max(lin[1], lin[2])))
	return l, l * s
}

func maxChroma(a, b, lightness, cuspL, cuspC float64) float64 {
	if lightness <= cuspL {
		return cuspC * lightness / cuspL
	}
	t := cuspC * (lightness - 1) / (cuspL - 1)
	slopes := lmsSlopes(a, b)
	var lms, cubes, first, second vec3
	for i := 0; i < 3; i++ {
		lms[i] = lightness + t*slopes[i]
		cubes[i] = lms[i] * lms[i] * lms[i]
		first[i] = 3 * slopes[i] * lms[i] * lms[i]
		second[i] = 6 * slopes[i] * slopes[i] * lms[i]
	}
	dot := func(row vec3, v vec3) float64 { return row[0]*v[0] + row[1]*v[1] + row[2]*v[2] }
	step := math.MaxFloat64
	for _, row := range lmsToLinearSrgb {
		f := dot(row, cubes) - 1
		f1 := dot(row, first)
		f2 := dot(row, second)
		u := f1 / (f1*f1 - 0.5*f*f2)
		s := math.MaxFloat64
		if u >= 0 {
			s = -f * u
		}
		step = math.Min(step, s)
	}
	return t + step
}

func chromaStops(L, a, b float64) (float64, float64, float64) {
	cuspL, cuspC := okCusp(a, b)
	cMax := maxChroma(a, b, L, cuspL, cuspC)
	k := cMax / math.Min(L*(cuspC/cuspL), (1-L)*(cuspC/(1-cuspL)))
	midS := 0.11516993 + 1/(7.4477897+4.1590124*b+a*(-2.19557347+1.75198401*b+a*(-2.13704948-10.02301043*b+a*(-4.24894561+5.38770819*b+4.69891013*a))))
	midT := 0.11239642 + 1/(1.6132032-0.68124379*b+a*(0.40370612+0.90148123*b+a*(-0.27087943+0.6122399*b+a*(0.00299215-0.45399568*b-0.14661872*a))))
	cMid := 0.9 * k * math.Sqrt(math.Sqrt(1/(1/math.Pow(L*midS, 4)+1/math.Pow((1-L)*midT, 4))))
	c0 := math.Sqrt(1 / (1/math.Pow(L*0.4, 2) + 1/math.Pow((1-L)*0.8, 2)))
	return c0, cMid, cMax
}

// okhslToRgb converts OKHSL (hue in degrees, saturation and lightness 0-1)
// to sRGB, clipping out-of-gamut channels.
func okhslToRgb(hue, saturation, lightness float64) rgb {
	L := okhslToOklabLightness(lightness)
	lab := vec3{L, 0, 0}
	if L > 0 && L < 1 && saturation > 0 {
		angle := 2 * math.Pi * math.Mod(math.Mod(hue, 360)+360, 360) / 360
		a, b := math.Cos(angle), math.Sin(angle)
		c0, cMid, cMax := chromaStops(L, a, b)
		var chroma float64
		if saturation < 0.8 {
			t := 1.25 * saturation
			k1 := 0.8 * c0
			chroma = t * k1 / (1 - (1-k1/cMid)*t)
		} else {
			t := 5 * (saturation - 0.8)
			k1 := 0.2 * cMid * cMid * 1.25 * 1.25 / c0
			chroma = cMid + t*k1/(1-(1-k1/(cMax-cMid))*t)
		}
		lab = vec3{L, chroma * a, chroma * b}
	}
	return linearSrgbToRgb(oklabToLinearSrgb(lab))
}

type okhsl struct{ h, s, l float64 }

// rgbToOkhsl converts sRGB to OKHSL (hue 0 for grays).
func rgbToOkhsl(c rgb) okhsl {
	lab := rgbToOklab(c)
	chroma := math.Hypot(lab[1], lab[2])
	lightness := oklabToOkhslLightness(lab[0])
	if chroma < 1e-9 || lightness <= 0 || lightness >= 1 {
		return okhsl{0, 0, lightness}
	}
	hue := math.Mod(math.Atan2(lab[2], lab[1])*180/math.Pi+360, 360)
	c0, cMid, cMax := chromaStops(lab[0], lab[1]/chroma, lab[2]/chroma)
	var s float64
	if chroma < cMid {
		k1 := 0.8 * c0
		s = 0.8 * (chroma / (k1 + (1-k1/cMid)*chroma))
	} else {
		k1 := 0.2 * cMid * cMid * 1.25 * 1.25 / c0
		offset := chroma - cMid
		s = 0.8 + 0.2*(offset/(k1+(1-k1/(cMax-cMid))*offset))
	}
	return okhsl{hue, math.Min(1, math.Max(0, s)), lightness}
}

type oklch struct{ l, c, h float64 }

func rgbToOklch(c rgb) oklch {
	lab := rgbToOklab(c)
	return oklch{lab[0], math.Hypot(lab[1], lab[2]), math.Mod(math.Atan2(lab[2], lab[1])*180/math.Pi+360, 360)}
}

func inSrgbGamut(lin vec3) bool {
	const eps = 1e-7
	for _, v := range lin {
		if v < -eps || v > 1+eps {
			return false
		}
	}
	return true
}

// oklchToRgb maps an OKLCH colour into sRGB, reducing chroma at a fixed hue
// until it fits (pi-tui colors.js).
func oklchToRgb(c oklch) rgb {
	h := math.Mod(math.Mod(c.h, 360)+360, 360)
	rad := h * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	at := func(chroma float64) vec3 { return oklabToLinearSrgb(vec3{c.l, chroma * cos, chroma * sin}) }
	direct := at(c.c)
	if inSrgbGamut(direct) {
		return linearSrgbToRgb(direct)
	}
	lin := at(0)
	low, high := 0.0, c.c
	for i := 0; i < 20; i++ {
		chroma := (low + high) / 2
		if candidate := at(chroma); inSrgbGamut(candidate) {
			low, lin = chroma, candidate
		} else {
			high = chroma
		}
	}
	return linearSrgbToRgb(lin)
}
