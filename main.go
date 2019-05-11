package main

import (
    "os"
    "time"
    "bytes"
    "image"
    "image/color"
    "image/png"
    "io/ioutil"

    "github.com/golang/freetype/truetype"
    "github.com/llgcode/draw2d"
    "github.com/llgcode/draw2d/draw2dimg"
)

func makeImage() *image.RGBA {
// Load and register the background image
    inuImage, err := draw2dimg.LoadFromPngFile("inu.png")
    if err != nil {
        panic(err)
    }

    // Load and register the font
    fontBytes, err := ioutil.ReadFile("ocr.ttf")
    if err != nil {
        panic(err)
    }
    font, err := truetype.Parse(fontBytes)
    if err != nil {
        panic(err)
    }
    fontData := draw2d.FontData{
        Name: "ocr",
        Family: draw2d.FontFamilySerif,
        Style: draw2d.FontStyleNormal,
    }
    draw2d.RegisterFont(fontData, font)

    // Format current time
    now := time.Now()
    nowStr := now.Format("15:04")

    // Initialize the graphic context on an RGBA image
    output := image.NewRGBA(image.Rect(0, 0, 512, 512))
    gc := draw2dimg.NewGraphicContext(output)

    // Set some properties
    gc.SetFontData(fontData)
    gc.SetFontSize(28)

    // Draw the background image first
    gc.DrawImage(inuImage)

    // Write some text
    gc.Save()
    gc.ComposeMatrixTransform(draw2d.Matrix{
        1.0, -0.02,
        -0.14, 1.0,
        301.0, 190.0,
    })
    gc.SetFillColor(color.RGBA{0x5d, 0xd8, 0xea, 0xff})
    gc.FillString(nowStr)
    gc.Restore()

    return output
}

func main() {
    image := makeImage()

    buffer := bytes.NewBuffer(make([]byte, 0))
    err := png.Encode(buffer, image)
    if err != nil {
        panic(err)
    }

    file, err := os.Create("out.png")
    if err != nil {
        panic(err)
    }
    defer file.Close()
    buffer.WriteTo(file)
}
