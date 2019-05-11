package main

import (
    "os"
    "fmt"
    "time"
    "bytes"
    "image"
    "image/color"
    "image/png"
    "io/ioutil"
    "net/http"
    "mime/multipart"

    "github.com/golang/freetype/truetype"
    "github.com/llgcode/draw2d"
    "github.com/llgcode/draw2d/draw2dimg"
)

type SlackTokenNotSet struct{}

func (SlackTokenNotSet) Error() string {
    return "Slack token not set"
}

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

func makeRequest(buffer *bytes.Buffer, token string) error {
    body := new(bytes.Buffer)
    writer := multipart.NewWriter(body)

    // Add the token
    tokenField, err := writer.CreateFormField("token")
    if err != nil {
        return err
    }
    tokenField.Write([]byte(token))

    // Add the image
    part, err := writer.CreateFormFile("image", "inu.png")
    if err != nil {
        return err
    }
    buffer.WriteTo(part)

    writerContentType := writer.FormDataContentType()

    err = writer.Close()
    if err != nil {
        return err
    }

    request, err := http.NewRequest("POST", "https://slack.com/api/users.setPhoto", body)
    if err != nil {
        return err
    }
    request.Header.Add("User-Agent", "curl/7.64.1")
    request.Header.Add("Accept", "*/* ")
    request.Header.Add("Content-Type", writerContentType)

    client := &http.Client{}
    resp, err := client.Do(request)
    if err != nil {
        return err
    } else {
        var bodyContent []byte
        resp.Body.Read(bodyContent)
        resp.Body.Close()
        fmt.Println(resp.StatusCode)
        fmt.Println(resp.Header)
        fmt.Println(bodyContent)
        fmt.Println(resp.Request.ContentLength)
        fmt.Println(resp.Request.Header)
    }

    return nil
}

func updateSlackPicture() {
    // Generate an image of CyberInu with time on glasses
    image := makeImage()

    // Encode the image to a PNG
    buffer := bytes.NewBuffer(make([]byte, 0))
    err := png.Encode(buffer, image)
    if err != nil {
        panic(err)
    }

    // Retrieve slack token
    var token string
    if len(os.Args) >= 2 {
        // Try to retrieve token from passed arguments first
        token = os.Args[1]
    }
    if token == "" {
        // If token not available in passed arguments
        // try to retrieve it from environment variables
        token = os.Getenv("SLACK_TOKEN")
    }
    if token == "" {
        // Slack token not set. Abort...
        panic(SlackTokenNotSet{})
    }

    // Make request to update slack profile picture
    err = makeRequest(buffer, token)
    if err != nil {
        panic(err)
    }
}

func main() {
    aminute, err := time.ParseDuration("1m")
    if err != nil {
        panic(err)
    }

    for {
        updateSlackPicture()
        time.Sleep(aminute)
    }
}
