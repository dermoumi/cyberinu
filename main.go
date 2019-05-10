package main

import (
    "os"
    "image"
    "time"
    "image/color"
    "io/ioutil"

    "github.com/golang/freetype/truetype"
    "github.com/llgcode/draw2d"
    "github.com/llgcode/draw2d/draw2dimg"
)

func main() {
    // Load and register the background image
    infile, err := os.Open("inu.png")
    if err != nil {
        panic(err)
    }
    defer infile.Close()

    inuImage, _, err := image.Decode(infile)
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
    dest := image.NewRGBA(inuImage.Bounds())
    gc := draw2dimg.NewGraphicContext(dest)

    // Set some properties
    gc.SetFillColor(color.RGBA{0x5d, 0xd8, 0xea, 0xff})
    // gc.SetFillColor(color.RGBA{0xff, 0xcc, 0x00, 0xff})
    gc.SetFontData(fontData)
    gc.SetFontSize(32)

    // Draw the background image first
    gc.DrawImage(inuImage)

    // Write some text
    gc.Save()
    gc.ComposeMatrixTransform(draw2d.Matrix{
        1.0, -0.02,
        -0.10, 1.0,
        412.0, 272.0,
    })
    gc.FillString(nowStr)
    gc.Restore()

    // Save to file
    draw2dimg.SaveToPngFile("out.png", dest)
}
