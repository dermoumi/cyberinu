package main

import (
    "bytes"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "image"
    "image/color"
    "image/png"
    "io"
    "io/ioutil"
    "log"
    "math/rand"
    "mime/multipart"
    "net/http"
    "os"
    "path"
    "time"

    "github.com/golang/freetype/truetype"
    "github.com/llgcode/draw2d"
    "github.com/llgcode/draw2d/draw2dimg"
)

const ASSETS_DIR = "assets"

type AppConfig struct {
    SlackToken     string
    UpdateInterval time.Duration
    SecondsOffset  int
    LogFile        string
    OutputFile     string
    ModelIndex     int
}

type InvalidFlagValue struct {
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

type Model struct {
    Filename  string
    Font      string
    FontSize  float64
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

var errEmptyModelList = errors.New("Model list is empty")

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

    // Make sure there's at least one model
    if len(loadedData) == 0 {
        return nil, errEmptyModelList
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

        font, ok := modelMap["font"].(string)
        if !ok {
            return nil, errInvalidFormat
        }

        fontSize, ok := modelMap["fontSize"].(float64)
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
            Filename:  filename,
            Font:      font,
            FontSize:  fontSize,
            Color:     color,
            Transform: draw2d.Matrix(transform),
        }
    }

    return models, nil
}

func makeImage(timeStr string, model *Model) (*image.RGBA, error) {
    // Load and register the background image
    inuImage, err := draw2dimg.LoadFromPngFile(path.Join(ASSETS_DIR, model.Filename))
    if err != nil {
        return nil, err
    }

    // Load and register the font
    fontBytes, err := ioutil.ReadFile(path.Join(ASSETS_DIR, model.Font))
    if err != nil {
        return nil, err
    }
    font, err := truetype.Parse(fontBytes)
    if err != nil {
        return nil, err
    }
    fontData := draw2d.FontData{
        Name:   model.Font,
        Family: draw2d.FontFamilySerif,
        Style:  draw2d.FontStyleNormal,
    }
    draw2d.RegisterFont(fontData, font)

    // Initialize the graphic context on an RGBA image
    output := image.NewRGBA(image.Rect(0, 0, 512, 512))
    gc := draw2dimg.NewGraphicContext(output)

    // Set some properties
    gc.SetFontData(fontData)
    gc.SetFontSize(model.FontSize)

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
            Headers:    resp.Header,
            Body:       bodyString,
        }
    }

    return nil
}

var errModelIndexOutOfRange = errors.New("Model index out of range")

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
    if config.ModelIndex >= 0 {
        // If the model index is already specified, choose it directly
        if config.ModelIndex >= modelCount {
            return errModelIndexOutOfRange
        }
        model = &models[config.ModelIndex]
    } else if modelCount == 1 {
        // If there's only one model, choose it directly
        model = &models[0]
    } else {
        // If there's more than one model and none was specified, choose randomly
        index := rand.Intn(modelCount)
        model = &models[index]
    }

    // Generate an image of CyberInu with time on glasses
    image, err := makeImage(nowStr, model)
    if err != nil {
        return err
    }

    // If output file is defined, save to PNG file and leave
    if config.OutputFile != "" {
        return draw2dimg.SaveToPngFile(config.OutputFile, image)
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
    slackTokenPtr := flag.String("slack-token", "",
        "Slack token")
    updateIntervalPtr := flag.Duration("update-interval", defaultUpdateInterval,
        "Update interval")
    secondsOffsetPtr := flag.Int("seconds-offset", 30,
        "Seconds after which we generate picture for the next minute")
    logFilePtr := flag.String("log-file", "",
        "File to log to")
    outputFilePtr := flag.String("output", "",
        "Generate the image and save it as a file instead of uploading to slack")
    modelIndexPtr := flag.Int("model-index", -1,
        "Model index to render")

    // Parse flags
    flag.Parse()

    // Create config
    config := AppConfig{
        *slackTokenPtr,
        *updateIntervalPtr,
        *secondsOffsetPtr,
        *logFilePtr,
        *outputFilePtr,
        *modelIndexPtr,
    }

    // Add missing flags from env
    if config.SlackToken == "" {
        config.SlackToken = os.Getenv("SLACK_TOKEN")
    }

    // Make sure all required flags are there
    if config.OutputFile == "" && config.SlackToken == "" {
        return nil, InvalidFlagValue{
            Flag:  "slack-token",
            Value: config.SlackToken,
        }
    }

    return &config, nil
}

func main() {
    // Seed random number generator
    rand.Seed(time.Now().UTC().UnixNano())

    // Setup logger
    log.SetOutput(os.Stdout)

    // Get flags
    config, err := parseFlags()
    if err != nil {
        log.Panic(err)
    }

    // Setup external log file if specified
    if config.LogFile != "" {
        logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0644)
        if err != nil {
            log.Printf("ERROR: opening log file '%s' for writing. %s", config.LogFile, err)
        }
        defer logFile.Close()

        log.SetOutput(io.MultiWriter(os.Stdout, logFile))
        log.Printf("Setup file '%s' as log file", config.LogFile)
    }

    // If output file is specified, do that and quit
    if config.OutputFile != "" {
        err = updateSlackPicture(config)
        if err != nil {
            log.Fatal(err)
        }

        log.Printf("File saved to '%s'", config.OutputFile)
        return
    }

    // Start the main loop
    for {
        err = updateSlackPicture(config)
        if err != nil {
            log.Printf("ERROR: %s", err)
        }

        time.Sleep(config.UpdateInterval)
    }
}
