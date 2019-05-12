package main

import (
    "os"
    "fmt"
    "log"
    "time"
    "flag"
    "path"
    "bytes"
    "errors"
    "image"
    "image/png"
    "image/color"
    "math/rand"
    "io/ioutil"
    "net/http"
    "encoding/json"
    "mime/multipart"

    "github.com/golang/freetype/truetype"
    "github.com/llgcode/draw2d"
    "github.com/llgcode/draw2d/draw2dimg"
)

type AppConfig struct{
    SlackToken          string
    UpdateInterval      time.Duration
    SecondsOffset       int
}

type InvalidFlagValue struct{
    Flag  string
    Value string
}

func (err InvalidFlagValue) Error() string {
    return fmt.Sprintf("Invalid value '%v' for '%s'", err.Value, err.Flag)
}

type Non200StatusCode struct {
    StatusCode int
    Headers    http.Header
    Body       string
}

func (err Non200StatusCode) Error() string {
    return fmt.Sprintf("%v", err)
}

type Model struct{
    Filename  string
    Color     color.RGBA
    Transform draw2d.Matrix
}

// https://stackoverflow.com/questions/54197913/parse-hex-string-to-image-color
var errInvalidFormat = errors.New("invalid format")
func parseHexColorFast(s string) (c color.RGBA, err error) {
    c.A = 0xff

    if s[0] != '#' {
        return c, errInvalidFormat
    }

    hexToByte := func(b byte) byte {
        switch {
        case b >= '0' && b <= '9':
            return b - '0'
        case b >= 'a' && b <= 'f':
            return b - 'a' + 10
        case b >= 'A' && b <= 'F':
            return b - 'A' + 10
        }
        err = errInvalidFormat
        return 0
    }

    switch len(s) {
    case 7:
        c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
        c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
        c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
    case 4:
        c.R = hexToByte(s[1]) * 17
        c.G = hexToByte(s[2]) * 17
        c.B = hexToByte(s[3]) * 17
    default:
        err = errInvalidFormat
    }
    return
}

func loadModels(filename string) ([]Model, error) {
    jsonBytes, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    var loadedData []interface{}
    err = json.Unmarshal(jsonBytes, &loadedData)
    if err != nil {
        return nil, err
    }

    models := make([]Model, len(loadedData))
    for i, modelMapInterface := range loadedData {
        modelMap, ok := modelMapInterface.(map[string]interface{})
        if !ok {
            return nil, errInvalidFormat
        }

        filename, ok := modelMap["filename"].(string)
        if !ok {
            return nil, errInvalidFormat
        }

        colorStr, ok := modelMap["color"].(string)
        if !ok {
            return nil, errInvalidFormat
        }
        color, err := parseHexColorFast(colorStr)
        if err != nil {
            return nil, err
        }

        transformInterface, ok := modelMap["transform"].([]interface{})
        if !ok {
            return nil, errInvalidFormat
        }

        var transform [6]float64
        for j, value := range transformInterface {
            transform[j], ok = value.(float64)
            if !ok {
                return nil, errInvalidFormat
            }
        }

        models[i] = Model{
            Filename: filename,
            Color: color,
            Transform: draw2d.Matrix(transform),
        }
    }

    return models, nil
}

func makeImage(timeStr string, model *Model) (*image.RGBA, error) {
// Load and register the background image
    inuImage, err := draw2dimg.LoadFromPngFile(model.Filename)
    if err != nil {
        return nil, err
    }

    // Load and register the font
    fontBytes, err := ioutil.ReadFile("ocr.ttf")
    if err != nil {
        return nil, err
    }
    font, err := truetype.Parse(fontBytes)
    if err != nil {
        return nil, err
    }
    fontData := draw2d.FontData{
        Name: "ocr",
        Family: draw2d.FontFamilySerif,
        Style: draw2d.FontStyleNormal,
    }
    draw2d.RegisterFont(fontData, font)

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
    gc.ComposeMatrixTransform(model.Transform)
    gc.SetFillColor(model.Color)
    gc.FillString(timeStr)
    gc.Restore()

    return output, nil
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
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        var bodyString string
        bodyBytes, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            bodyString = "<UNABLE TO READ RESPONSE BODY>"
        }
        bodyString = string(bodyBytes)

        return Non200StatusCode{
            StatusCode: resp.StatusCode,
            Headers: resp.Header,
            Body: bodyString,
        }
    }

    return nil
}

func updateSlackPicture(config *AppConfig) error {
    // Format current time
    now := time.Now()
    if now.Second() > config.SecondsOffset {
        oneMinute, err := time.ParseDuration("1m")
        if err == nil {
            now = now.Add(oneMinute)
        }
    }
    nowStr := now.Format("15:04")

    // Select a model to use
    models, err := loadModels("models.json")
    if err != nil {
        return err
    }

    // Select a random model if modelCount > 1
    var model *Model
    modelCount := len(models)
    if modelCount == 0 {
        // TODO: return error
        return nil
    } else if modelCount == 1 {
        model = &models[0]
    } else {
        index := rand.Intn(modelCount - 1)
        model = &models[index]
    }

    // Generate an image of CyberInu with time on glasses
    image, err := makeImage(nowStr, model)
    if err != nil {
        return err
    }

    // Encode the image to a PNG
    buffer := bytes.NewBuffer(make([]byte, 0))
    err = png.Encode(buffer, image)
    if err != nil {
        return err
    } else {
        log.Printf("Generated new CyberInu with time: %s.", nowStr)
    }

    // Make request to update slack profile picture
    err = makeRequest(buffer, config.SlackToken)
    if err != nil {
        return err
    } else {
        log.Printf("Slack profile picture successfully updated.")
    }

    return nil
}

func parseFlags() (*AppConfig, error) {
    // Prepare default durations
    defaultUpdateInterval, err := time.ParseDuration("1m")
    if err != nil {
        return nil, err
    }

    // Setup flags
    slackTokenPtr := flag.String("slack-token", "", "Slack token")
    updateIntervalPtr := flag.Duration("update-interval", defaultUpdateInterval, "Update interval")
    secondsOffsetPtr := flag.Int("seconds-offset", 30,
                                 "Seconds after which we generate picture for the next minute")

    // Parse flags
    flag.Parse()

    // Create config
    config := AppConfig{
        *slackTokenPtr,
        *updateIntervalPtr,
        *secondsOffsetPtr,
    }

    // Add missing flags from env
    if config.SlackToken == "" {
        config.SlackToken = os.Getenv("SLACK_TOKEN")
    }

    // Make sure all required flags are there
    if config.SlackToken == "" {
        return nil, InvalidFlagValue{
            Flag: "slack-token",
            Value: config.SlackToken,
        }
    }

    return &config, nil
}

func main() {
    // Setup logger
    // TODO: Add a MultiWriter to both stdout and a file
    log.SetOutput(os.Stdout)

    // Get flags
    config, err := parseFlags()
    if err != nil {
        log.Panic(err)
    }

    // Start the main loop
    for {
        err = updateSlackPicture(config)
        if err != nil {
            log.Print(err)
        }

        time.Sleep(config.UpdateInterval)
    }
}
