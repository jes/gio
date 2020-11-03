// SPDX-License-Identifier: Unlicense OR MIT

package paint

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// ImageOp sets the brush to an image.
//
// Note: the ImageOp may keep a reference to the backing image.
// See NewImageOp for details.
type ImageOp struct {
	// Rect is the section if the backing image to use.
	Rect image.Rectangle

	uniform bool
	color   color.RGBA
	src     *image.RGBA

	// handle is a key to uniquely identify this ImageOp
	// in a map of cached textures.
	handle interface{}
}

// ColorOp sets the brush to a constant color.
type ColorOp struct {
	Color color.RGBA
}

// LinearGradientOp sets the brush to a gradient starting at stop1 with color1 and
// ending at stop2 with color2.
type LinearGradientOp struct {
	Stop1  f32.Point
	Color1 color.RGBA
	Stop2  f32.Point
	Color2 color.RGBA
}

// PaintOp fills an area with the current brush, respecting the
// current clip path and transformation.
type PaintOp struct {
	// Rect is the destination area to paint. If necessary, the brush is
	// scaled to cover the rectangle area.
	Rect f32.Rectangle
}

// NewImageOp creates an ImageOp backed by src. See
// gioui.org/io/system.FrameEvent for a description of when data
// referenced by operations is safe to re-use.
//
// NewImageOp assumes the backing image is immutable, and may cache a
// copy of its contents in a GPU-friendly way. Create new ImageOps to
// ensure that changes to an image is reflected in the display of
// it.
func NewImageOp(src image.Image) ImageOp {
	switch src := src.(type) {
	case *image.Uniform:
		col := color.RGBAModel.Convert(src.C).(color.RGBA)
		return ImageOp{
			uniform: true,
			color:   col,
		}
	case *image.RGBA:
		bounds := src.Bounds()
		if bounds.Min == (image.Point{}) && src.Stride == bounds.Dx()*4 {
			return ImageOp{
				Rect:   src.Bounds(),
				src:    src,
				handle: new(int),
			}
		}
	}

	sz := src.Bounds().Size()
	// Copy the image into a GPU friendly format.
	dst := image.NewRGBA(image.Rectangle{
		Max: sz,
	})
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return ImageOp{
		Rect:   dst.Bounds(),
		src:    dst,
		handle: new(int),
	}
}

func (i ImageOp) Size() image.Point {
	if i.src == nil {
		return image.Point{}
	}
	return i.src.Bounds().Size()
}

func (i ImageOp) Add(o *op.Ops) {
	if i.uniform {
		ColorOp{
			Color: i.color,
		}.Add(o)
		return
	}
	data := o.Write(opconst.TypeImageLen, i.src, i.handle)
	data[0] = byte(opconst.TypeImage)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(i.Rect.Min.X))
	bo.PutUint32(data[5:], uint32(i.Rect.Min.Y))
	bo.PutUint32(data[9:], uint32(i.Rect.Max.X))
	bo.PutUint32(data[13:], uint32(i.Rect.Max.Y))
}

func (c ColorOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypeColorLen)
	data[0] = byte(opconst.TypeColor)
	data[1] = c.Color.R
	data[2] = c.Color.G
	data[3] = c.Color.B
	data[4] = c.Color.A
}

func (c LinearGradientOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypeLinearGradientLen)
	data[0] = byte(opconst.TypeLinearGradient)

	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(c.Stop1.X))
	bo.PutUint32(data[5:], math.Float32bits(c.Stop1.Y))
	bo.PutUint32(data[9:], math.Float32bits(c.Stop2.X))
	bo.PutUint32(data[13:], math.Float32bits(c.Stop2.Y))

	data[17+0] = c.Color1.R
	data[17+1] = c.Color1.G
	data[17+2] = c.Color1.B
	data[17+3] = c.Color1.A
	data[21+0] = c.Color2.R
	data[21+1] = c.Color2.G
	data[21+2] = c.Color2.B
	data[21+3] = c.Color2.A
}

func (d PaintOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypePaintLen)
	data[0] = byte(opconst.TypePaint)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(d.Rect.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(d.Rect.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(d.Rect.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(d.Rect.Max.Y))
}

// FillShape fills the clip shape with a color.
func FillShape(ops *op.Ops, c color.RGBA, shape clip.Op) {
	defer op.Push(ops).Pop()
	shape.Add(ops)
	Fill(ops, c)
}

// Fill paints an infinitely large plane with the provided color. It
// is intended to be used with a clip.Op already in place to limit
// the painted area. Use FillShape unless you need to paint several
// times within the same clip.Op.
func Fill(ops *op.Ops, c color.RGBA) {
	defer op.Push(ops).Pop()
	ColorOp{Color: c}.Add(ops)
	inf := float32(math.Inf(+1))
	PaintOp{
		Rect: f32.Rectangle{
			Min: f32.Pt(-inf, -inf),
			Max: f32.Pt(+inf, +inf),
		},
	}.Add(ops)
}
