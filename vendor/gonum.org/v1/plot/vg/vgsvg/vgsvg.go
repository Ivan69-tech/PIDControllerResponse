// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vgsvg uses svgo (github.com/ajstarks/svgo)
// as a backend for vg.
//
// By default, gonum/plot uses the Liberation fonts.
// When embedding was not requested during plot creation, it may happen that
// the generated SVG plot may not display well if the Liberation fonts are not
// available to the program displaying the SVG plot.
// See gonum.org/v1/plot/vg/vgsvg#Example_standardFonts for how to work around
// this issue.
//
// Alternatively, users may want to install the Liberation fonts on their system:
//   - https://en.wikipedia.org/wiki/Liberation_fonts
package vgsvg // import "gonum.org/v1/plot/vg/vgsvg"

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"strings"

	svgo "github.com/ajstarks/svgo"
	xfnt "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"

	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

func init() {
	draw.RegisterFormat("svg", func(w, h vg.Length) vg.CanvasWriterTo {
		return New(w, h)
	})
}

// pr is the precision to use when outputting float64s.
const pr = 5

const (
	// DefaultWidth and DefaultHeight are the default canvas
	// dimensions.
	DefaultWidth  = 4 * vg.Inch
	DefaultHeight = 4 * vg.Inch
)

// Canvas implements the vg.Canvas interface, drawing to a SVG document.
//
// By default, fonts used by the canvas are not embedded in the produced
// SVG document. This results in smaller but less portable SVG plots.
// Users wanting completely portable SVG documents should create SVG canvases
// with the EmbedFonts function.
type Canvas struct {
	svg  *svgo.SVG
	w, h vg.Length

	hdr   *bytes.Buffer // hdr is the SVG prelude, it may contain embedded fonts.
	buf   *bytes.Buffer // buf is the SVG document.
	stack []context

	// Switch to embed fonts in SVG file.
	// The default is to *not* embed fonts.
	// Embedding fonts makes the SVG file larger but also more portable.
	embed bool
	fonts map[string]struct{} // set of already embedded fonts
}

type context struct {
	color      color.Color
	dashArray  []vg.Length
	dashOffset vg.Length
	lineWidth  vg.Length
	gEnds      int
}

type option func(*Canvas)

// UseWH specifies the width and height of the canvas.
func UseWH(w, h vg.Length) option {
	return func(c *Canvas) {
		if w <= 0 || h <= 0 {
			panic("vgsvg: w and h must both be > 0")
		}
		c.w = w
		c.h = h
	}
}

// EmbedFonts specifies whether fonts should be embedded inside
// the SVG canvas.
func EmbedFonts(v bool) option {
	return func(c *Canvas) {
		c.embed = v
	}
}

// New returns a new image canvas.
func New(w, h vg.Length) *Canvas {
	return NewWith(UseWH(w, h))
}

// NewWith returns a new image canvas created according to the specified
// options. The currently accepted options is UseWH. If size is not
// specified, the default is used.
func NewWith(opts ...option) *Canvas {
	buf := new(bytes.Buffer)
	c := &Canvas{
		svg:   svgo.New(buf),
		w:     DefaultWidth,
		h:     DefaultHeight,
		hdr:   new(bytes.Buffer),
		buf:   buf,
		stack: []context{{}},
		embed: false,
		fonts: make(map[string]struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	// This is like svg.Start, except it uses floats
	// and specifies the units.
	fmt.Fprintf(c.hdr, `<?xml version="1.0"?>
<!-- Generated by SVGo and Plotinum VG -->
<svg width="%.*gpt" height="%.*gpt" viewBox="0 0 %.*g %.*g"
	xmlns="http://www.w3.org/2000/svg"
	xmlns:xlink="http://www.w3.org/1999/xlink">`+"\n",
		pr, c.w,
		pr, c.h,
		pr, c.w,
		pr, c.h,
	)

	if c.embed {
		fmt.Fprintf(c.hdr, "<defs>\n\t<style>\n")
	}

	// Swap the origin to the bottom left.
	// This must be matched with a </g> when saving,
	// before the closing </svg>.
	c.svg.Gtransform(fmt.Sprintf("scale(1, -1) translate(0, -%.*g)", pr, c.h.Points()))

	vg.Initialize(c)
	return c
}

func (c *Canvas) Size() (w, h vg.Length) {
	return c.w, c.h
}

func (c *Canvas) context() *context {
	return &c.stack[len(c.stack)-1]
}

func (c *Canvas) SetLineWidth(w vg.Length) {
	c.context().lineWidth = w
}

func (c *Canvas) SetLineDash(dashes []vg.Length, offs vg.Length) {
	c.context().dashArray = dashes
	c.context().dashOffset = offs
}

func (c *Canvas) SetColor(clr color.Color) {
	c.context().color = clr
}

func (c *Canvas) Rotate(rot float64) {
	rot = rot * 180 / math.Pi
	c.svg.Rotate(rot)
	c.context().gEnds++
}

func (c *Canvas) Translate(pt vg.Point) {
	c.svg.Gtransform(fmt.Sprintf("translate(%.*g, %.*g)", pr, pt.X.Points(), pr, pt.Y.Points()))
	c.context().gEnds++
}

func (c *Canvas) Scale(x, y float64) {
	c.svg.ScaleXY(x, y)
	c.context().gEnds++
}

func (c *Canvas) Push() {
	top := *c.context()
	top.gEnds = 0
	c.stack = append(c.stack, top)
}

func (c *Canvas) Pop() {
	for i := 0; i < c.context().gEnds; i++ {
		c.svg.Gend()
	}
	c.stack = c.stack[:len(c.stack)-1]
}

func (c *Canvas) Stroke(path vg.Path) {
	if c.context().lineWidth.Points() <= 0 {
		return
	}
	c.svg.Path(c.pathData(path),
		style(elm("fill", "#000000", "none"),
			elm("stroke", "none", colorString(c.context().color)),
			elm("stroke-opacity", "1", opacityString(c.context().color)),
			elmf("stroke-width", "1", "%.*g", pr, c.context().lineWidth.Points()),
			elm("stroke-dasharray", "none", dashArrayString(c)),
			elmf("stroke-dashoffset", "0", "%.*g", pr, c.context().dashOffset.Points())))
}

func (c *Canvas) Fill(path vg.Path) {
	c.svg.Path(c.pathData(path),
		style(elm("fill", "#000000", colorString(c.context().color)),
			elm("fill-opacity", "1", opacityString(c.context().color))))
}

func (c *Canvas) pathData(path vg.Path) string {
	buf := new(bytes.Buffer)
	var x, y float64
	for _, comp := range path {
		switch comp.Type {
		case vg.MoveComp:
			fmt.Fprintf(buf, "M%.*g,%.*g", pr, comp.Pos.X.Points(), pr, comp.Pos.Y.Points())
			x = comp.Pos.X.Points()
			y = comp.Pos.Y.Points()
		case vg.LineComp:
			fmt.Fprintf(buf, "L%.*g,%.*g", pr, comp.Pos.X.Points(), pr, comp.Pos.Y.Points())
			x = comp.Pos.X.Points()
			y = comp.Pos.Y.Points()
		case vg.ArcComp:
			r := comp.Radius.Points()
			sin, cos := math.Sincos(comp.Start)
			x0 := comp.Pos.X.Points() + r*cos
			y0 := comp.Pos.Y.Points() + r*sin
			if x0 != x || y0 != y {
				fmt.Fprintf(buf, "L%.*g,%.*g", pr, x0, pr, y0)
			}
			if math.Abs(comp.Angle) >= 2*math.Pi {
				x, y = circle(buf, c, &comp)
			} else {
				x, y = arc(buf, c, &comp)
			}
		case vg.CurveComp:
			switch len(comp.Control) {
			case 1:
				fmt.Fprintf(buf, "Q%.*g,%.*g,%.*g,%.*g",
					pr, comp.Control[0].X.Points(), pr, comp.Control[0].Y.Points(),
					pr, comp.Pos.X.Points(), pr, comp.Pos.Y.Points())
			case 2:
				fmt.Fprintf(buf, "C%.*g,%.*g,%.*g,%.*g,%.*g,%.*g",
					pr, comp.Control[0].X.Points(), pr, comp.Control[0].Y.Points(),
					pr, comp.Control[1].X.Points(), pr, comp.Control[1].Y.Points(),
					pr, comp.Pos.X.Points(), pr, comp.Pos.Y.Points())
			default:
				panic("vgsvg: invalid number of control points")
			}
			x = comp.Pos.X.Points()
			y = comp.Pos.Y.Points()
		case vg.CloseComp:
			buf.WriteString("Z")
		default:
			panic(fmt.Sprintf("vgsvg: unknown path component type: %d", comp.Type))
		}
	}
	return buf.String()
}

// circle adds circle path data to the given writer.
// Circles must be drawn using two arcs because
// SVG disallows the start and end point of an arc
// from being at the same location.
func circle(w io.Writer, c *Canvas, comp *vg.PathComp) (x, y float64) {
	angle := 2 * math.Pi
	if comp.Angle < 0 {
		angle = -2 * math.Pi
	}
	angle += remainder(comp.Angle, 2*math.Pi)
	if angle >= 4*math.Pi {
		panic("Impossible angle")
	}

	s0, c0 := math.Sincos(comp.Start + 0.5*angle)
	s1, c1 := math.Sincos(comp.Start + angle)

	r := comp.Radius.Points()
	x0 := comp.Pos.X.Points() + r*c0
	y0 := comp.Pos.Y.Points() + r*s0
	x = comp.Pos.X.Points() + r*c1
	y = comp.Pos.Y.Points() + r*s1

	fmt.Fprintf(w, "A%.*g,%.*g 0 %d %d %.*g,%.*g", pr, r, pr, r,
		large(angle/2), sweep(angle/2), pr, x0, pr, y0) //
	fmt.Fprintf(w, "A%.*g,%.*g 0 %d %d %.*g,%.*g", pr, r, pr, r,
		large(angle/2), sweep(angle/2), pr, x, pr, y)
	return
}

// remainder returns the remainder of x/y.
// We don't use math.Remainder because it
// seems to return incorrect values due to how
// IEEE defines the remainder operation…
func remainder(x, y float64) float64 {
	return (x/y - math.Trunc(x/y)) * y
}

// arc adds arc path data to the given writer.
// Arc can only be used if the arc's angle is
// less than a full circle, if it is greater then
// circle should be used instead.
func arc(w io.Writer, c *Canvas, comp *vg.PathComp) (x, y float64) {
	r := comp.Radius.Points()
	sin, cos := math.Sincos(comp.Start + comp.Angle)
	x = comp.Pos.X.Points() + r*cos
	y = comp.Pos.Y.Points() + r*sin
	fmt.Fprintf(w, "A%.*g,%.*g 0 %d %d %.*g,%.*g", pr, r, pr, r,
		large(comp.Angle), sweep(comp.Angle), pr, x, pr, y)
	return
}

// sweep returns the arc sweep flag value for
// the given angle.
func sweep(a float64) int {
	if a < 0 {
		return 0
	}
	return 1
}

// large returns the arc's large flag value for
// the given angle.
func large(a float64) int {
	if math.Abs(a) >= math.Pi {
		return 1
	}
	return 0
}

// FillString draws str at position pt using the specified font.
// Text passed to FillString is escaped with html.EscapeString.
func (c *Canvas) FillString(font font.Face, pt vg.Point, str string) {
	name := svgFontDescr(font)
	sty := style(
		name,
		elmf("font-size", "medium", "%.*gpx", pr, font.Font.Size.Points()),
		elm("fill", "#000000", colorString(c.context().color)),
	)
	if sty != "" {
		sty = "\n\t" + sty
	}
	fmt.Fprintf(
		c.buf,
		`<text x="%.*g" y="%.*g" transform="scale(1, -1)"%s>%s</text>`+"\n",
		pr, pt.X.Points(), pr, -pt.Y.Points(), sty, html.EscapeString(str),
	)

	if c.embed {
		c.embedFont(name, font)
	}
}

// DrawImage implements the vg.Canvas.DrawImage method.
func (c *Canvas) DrawImage(rect vg.Rectangle, img image.Image) {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		panic(fmt.Errorf("vgsvg: error encoding image to PNG: %+v", err))
	}
	str := "data:image/jpg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	rsz := rect.Size()
	min := rect.Min
	var (
		width  = rsz.X.Points()
		height = rsz.Y.Points()
		xmin   = min.X.Points()
		ymin   = min.Y.Points()
	)
	fmt.Fprintf(
		c.buf,
		`<image x="%v" y="%v" width="%v" height="%v" xlink:href="%s" %s />`+"\n",
		xmin,
		-ymin-height,
		width,
		height,
		str,
		// invert y so image is not upside-down
		`transform="scale(1, -1)"`,
	)
}

// svgFontDescr returns a SVG compliant font name from the provided font face.
func svgFontDescr(fnt font.Face) string {
	var (
		family  = svgFamilyName(fnt)
		variant = svgVariantName(fnt.Font.Variant)
		style   = svgStyleName(fnt.Font.Style)
		weight  = svgWeightName(fnt.Font.Weight)
	)

	o := "font-family:" + family + ";" +
		"font-variant:" + variant + ";" +
		"font-weight:" + weight + ";" +
		"font-style:" + style
	return o
}

func svgFamilyName(fnt font.Face) string {
	// https://developer.mozilla.org/en-US/docs/Web/SVG/Attribute/font-family
	var buf sfnt.Buffer
	name, err := fnt.Face.Name(&buf, sfnt.NameIDFamily)
	if err != nil {
		// this should never happen unless the underlying sfnt.Font data
		// is somehow corrupted.
		panic(fmt.Errorf(
			"vgsvg: could not extract family name from font %q: %+v",
			fnt.Font.Typeface,
			err,
		))
	}
	return name
}

func svgVariantName(v font.Variant) string {
	// https://developer.mozilla.org/en-US/docs/Web/SVG/Attribute/font-variant
	str := strings.ToLower(string(v))
	switch str {
	case "smallcaps":
		return "small-caps"
	case "mono", "monospace",
		"sans", "sansserif", "sans-serif",
		"serif":
		// handle mismatch between the meaning of gonum/plot/font.Font#Variant
		// and SVG's meaning for font-variant.
		// For SVG, mono, ... serif is encoded in the font-family attribute
		// whereas for gonum/plot it describes a variant among a collection of fonts.
		//
		// It shouldn't matter much if an invalid font-variant value is written
		// out (browsers will just ignore it; Firefox 98 and Chromium 91 do so.)
		return "normal"
	case "":
		return "none"
	default:
		return str
	}
}

func svgStyleName(sty xfnt.Style) string {
	// https://developer.mozilla.org/en-US/docs/Web/SVG/Attribute/font-style
	switch sty {
	case xfnt.StyleNormal:
		return "normal"
	case xfnt.StyleItalic:
		return "italic"
	case xfnt.StyleOblique:
		return "oblique"
	default:
		panic(fmt.Errorf("vgsvg: invalid font style %+v (v=%d)", sty, int(sty)))
	}
}

func svgWeightName(w xfnt.Weight) string {
	// see:
	//  https://developer.mozilla.org/en-US/docs/Web/SVG/Attribute/font-weight
	//  https://developer.mozilla.org/en-US/docs/Web/CSS/font-weight
	switch w {
	case xfnt.WeightThin:
		return "100"
	case xfnt.WeightExtraLight:
		return "200"
	case xfnt.WeightLight:
		return "300"
	case xfnt.WeightNormal:
		return "normal"
	case xfnt.WeightMedium:
		return "500"
	case xfnt.WeightSemiBold:
		return "600"
	case xfnt.WeightBold:
		return "bold"
	case xfnt.WeightExtraBold:
		return "800"
	case xfnt.WeightBlack:
		return "900"
	default:
		panic(fmt.Errorf("vgsvg: invalid font weight %+v (v=%d)", w, int(w)))
	}
}

func (c *Canvas) embedFont(name string, f font.Face) {
	if _, dup := c.fonts[name]; dup {
		return
	}
	c.fonts[name] = struct{}{}

	raw := new(bytes.Buffer)
	_, err := f.Face.WriteSourceTo(nil, raw)
	if err != nil {
		panic(fmt.Errorf("vg/vgsvg: could not read font raw data: %+v", err))
	}

	fmt.Fprintf(c.hdr, "\t\t@font-face{\n")
	fmt.Fprintf(c.hdr, "\t\t\tfont-family:%q;\n", svgFamilyName(f))
	fmt.Fprintf(c.hdr,
		"\t\t\tfont-variant:%s;font-weight:%s;font-style:%s;\n",
		svgVariantName(f.Font.Variant),
		svgWeightName(f.Font.Weight),
		svgStyleName(f.Font.Style),
	)

	fmt.Fprintf(
		c.hdr,
		"\t\t\tsrc: url(data:font/ttf;charset=utf-8;base64,%s) format(\"truetype\");\n",
		base64.StdEncoding.EncodeToString(raw.Bytes()),
	)
	fmt.Fprintf(c.hdr, "\t\t}\n")
}

type cwriter struct {
	w *bufio.Writer
	n int64
}

func (c *cwriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// WriteTo writes the canvas to an io.Writer.
func (c *Canvas) WriteTo(w io.Writer) (int64, error) {
	b := &cwriter{w: bufio.NewWriter(w)}

	if c.embed {
		fmt.Fprintf(c.hdr, "\t</style>\n</defs>\n")
	}

	_, err := c.hdr.WriteTo(b)
	if err != nil {
		return b.n, err
	}

	_, err = c.buf.WriteTo(b)
	if err != nil {
		return b.n, err
	}

	// Close the groups and svg in the output buffer
	// so that the Canvas is not closed and can be
	// used again if needed.
	for i := 0; i < c.nEnds(); i++ {
		_, err = fmt.Fprintln(b, "</g>")
		if err != nil {
			return b.n, err
		}
	}

	_, err = fmt.Fprintln(b, "</svg>")
	if err != nil {
		return b.n, err
	}

	return b.n, b.w.Flush()
}

// nEnds returns the number of group ends
// needed before the SVG is saved.
func (c *Canvas) nEnds() int {
	n := 1 // close the transform that moves the origin
	for _, ctx := range c.stack {
		n += ctx.gEnds
	}
	return n
}

// style returns a style string composed of
// all of the given elements.  If the elements
// are all empty then the empty string is
// returned.
func style(elms ...string) string {
	str := ""
	for _, e := range elms {
		if e == "" {
			continue
		}
		if str != "" {
			str += ";"
		}
		str += e
	}
	if str == "" {
		return ""
	}
	return "style=\"" + str + "\""
}

// elm returns a style element string with the
// given key and value.  If the value matches
// default then the empty string is returned.
func elm(key, def, f string) string {
	val := f
	if val == def {
		return ""
	}
	return key + ":" + val
}

// elmf returns a style element string with the
// given key and value.  If the value matches
// default then the empty string is returned.
func elmf(key, def, f string, vls ...interface{}) string {
	value := fmt.Sprintf(f, vls...)
	if value == def {
		return ""
	}
	return key + ":" + value
}

// dashArrayString returns a string representing the
// dash array specification.
func dashArrayString(c *Canvas) string {
	str := ""
	for i, d := range c.context().dashArray {
		str += fmt.Sprintf("%.*g", pr, d.Points())
		if i < len(c.context().dashArray)-1 {
			str += ","
		}
	}
	if str == "" {
		str = "none"
	}
	return str
}

// colorString returns the hexadecimal string representation of the color
func colorString(clr color.Color) string {
	if clr == nil {
		clr = color.Black
	}
	r, g, b, _a := clr.RGBA()
	a := 255.0 / float64(_a)
	return fmt.Sprintf("#%02X%02X%02X", int(float64(r)*a),
		int(float64(g)*a), int(float64(b)*a))
}

// opacityString returns the opacity value of the given color.
func opacityString(clr color.Color) string {
	if clr == nil {
		clr = color.Black
	}
	_, _, _, a := clr.RGBA()
	return fmt.Sprintf("%.*g", pr, float64(a)/math.MaxUint16)
}