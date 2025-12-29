package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "image/png"
    "net"
    "os"
    "path/filepath"
    "runtime"
    "strconv"
    "time"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/dialog"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/widget"
    "github.com/go-resty/resty/v2"
    "github.com/kbinani/screenshot"
)

// --- Configuration Struct ---
type AgentConfig struct {
    AgentID      uint   `json:"agent_id"`
    UserFullName string `json:"user_full_name"`
    Organization string `json:"organization"`
    AgreedToToS  bool   `json:"agreed_to_tos"`
}

var (
    client       = resty.New()
    serverURL    = "192.168.2.87:8080" // Update your IP here
    agentConfig  AgentConfig
    configPath   = "agent_config.json"
    
    // Concurrency Controls
    stopTracking = make(chan bool)
    isTracking   = false
    
    // UI Elements for updates
    statusLabel  *widget.Label
    previewImage *canvas.Image
)

func main() {
    a := app.New()
    w := a.NewWindow("Workplace Monitor Agent")
    w.Resize(fyne.NewSize(400, 600))

    // 1. Check if configuration exists (First Run Logic)
    if !loadConfig() {
        showPrivacyScreen(w)
    } else {
        showDashboard(w)
    }

    w.ShowAndRun()
}

// --- UI Screens ---

func showPrivacyScreen(w fyne.Window) {
    title := widget.NewLabelWithStyle("Privacy & Consent", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
    
    privacyText := widget.NewLabel("This application monitors your desktop activity,\n" +
        "including active windows and screenshots, for\n" +
        "productivity analysis.\n\n" +
        "Data is sent to your organization's server.")
    privacyText.Wrapping = fyne.TextWrapWord

    check := widget.NewCheck("I agree to the privacy policy and allow monitoring.", nil)
    
    nextBtn := widget.NewButton("Next", func() {
        if check.Checked {
            showRegistrationScreen(w)
        } else {
            dialog.ShowInformation("Required", "You must agree to proceed.", w)
        }
    })
    nextBtn.Disable()

    check.OnChanged = func(b bool) {
        if b {
            nextBtn.Enable()
        } else {
            nextBtn.Disable()
        }
    }

    w.SetContent(container.NewVBox(
        title,
        widget.NewSeparator(),
        privacyText,
        layout.NewSpacer(),
        check,
        nextBtn,
    ))
}

func showRegistrationScreen(w fyne.Window) {
    title := widget.NewLabelWithStyle("User Information", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
    
    nameEntry := widget.NewEntry()
    nameEntry.PlaceHolder = "Full Name"
    
    orgEntry := widget.NewEntry()
    orgEntry.PlaceHolder = "Organization Name"

    saveBtn := widget.NewButton("Save & Start", func() {
        if nameEntry.Text == "" || orgEntry.Text == "" {
            dialog.ShowError(fmt.Errorf("please fill all fields"), w)
            return
        }

        // Save to memory
        agentConfig.UserFullName = nameEntry.Text
        agentConfig.Organization = orgEntry.Text
        agentConfig.AgreedToToS = true
        
        // Register with server
        if err := registerSelf(); err != nil {
            dialog.ShowError(fmt.Errorf("server connection failed: %v", err), w)
            return
        }

        // Save config to file
        saveConfig()

        // Move to dashboard
        showDashboard(w)
    })

    w.SetContent(container.NewVBox(
        title,
        widget.NewSeparator(),
        widget.NewLabel("Your Name:"),
        nameEntry,
        widget.NewLabel("Organization:"),
        orgEntry,
        layout.NewSpacer(),
        saveBtn,
    ))
}

func showDashboard(w fyne.Window) {
    title := widget.NewLabelWithStyle("Agent Dashboard", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
    
    info := widget.NewLabel(fmt.Sprintf("User: %s\nOrg: %s", agentConfig.UserFullName, agentConfig.Organization))
    
    statusLabel = widget.NewLabel("Status: Idle")
    statusLabel.TextStyle = fyne.TextStyle{Bold: true}

    // Image Preview Place holder
    previewImage = canvas.NewImageFromResource(nil)
    previewImage.FillMode = canvas.ImageFillContain
    previewImage.SetMinSize(fyne.NewSize(300, 200))
    
    var startBtn *widget.Button
    var stopBtn *widget.Button

    startBtn = widget.NewButton("START MONITORING", func() {
        if isTracking { return }
        isTracking = true
        statusLabel.SetText("Status: MONITORING ACTIVE")
        startBtn.Disable()
        stopBtn.Enable()
        
        // Start background routines
        go runTrackingLoop()
    })
    startBtn.Importance = widget.HighImportance

    stopBtn = widget.NewButton("STOP MONITORING", func() {
        if !isTracking { return }
        // Signal stop
        stopTracking <- true
        isTracking = false
        statusLabel.SetText("Status: Stopped")
        startBtn.Enable()
        stopBtn.Disable()
    })
    stopBtn.Disable() // Initially disabled

    w.SetContent(container.NewVBox(
        title,
        info,
        widget.NewSeparator(),
        statusLabel,
        previewImage, // Shows latest screenshot
        layout.NewSpacer(),
        startBtn,
        stopBtn,
    ))
}

// --- Background Logic ---

func runTrackingLoop() {
    // Tickers
    activityTicker := time.NewTicker(1 * time.Second)
    screenshotTicker := time.NewTicker(60 * time.Second) // Set your interval here
    
    defer activityTicker.Stop()
    defer screenshotTicker.Stop()

    lastWindow := ""

    for {
        select {
        case <-stopTracking:
            return // Exit loop
        
        case <-activityTicker.C:
            currentWindow := getActiveWindowTitle()
            if currentWindow != "" && currentWindow != lastWindow {
                client.R().
                    SetBody(map[string]interface{}{
                        "agent_id": agentConfig.AgentID,
                        "window":   currentWindow,
                        "timestamp": time.Now(),
                    }).
                    Post(serverURL + "/api/activity")
                lastWindow = currentWindow
            }
        
        case <-screenshotTicker.C:
            captureAndUpload()
        }
    }
}

func captureAndUpload() {
    n := screenshot.NumActiveDisplays()
    if n <= 0 { return }

    // Just capture primary display for preview simplicity, but upload loop all
    for i := 0; i < n; i++ {
        bounds := screenshot.GetDisplayBounds(i)
        img, err := screenshot.CaptureRect(bounds)
        if err != nil { continue }
        
        var imageBuf bytes.Buffer
        png.Encode(&imageBuf, img)
        
        // Update GUI Preview (Thread Safe)
        if i == 0 {
            // We need to reload the image in the UI thread
            tmpImg := img // copy
            fyne.CurrentApp().Driver().CanvasForObject(previewImage).Refresh(previewImage)
            previewImage.Image = tmpImg
            previewImage.Refresh()
        }

        filename := fmt.Sprintf("screen_%d_%d.png", agentConfig.AgentID, i)
        
        go func(buf bytes.Buffer, fname string, displayIdx int) {
             client.R().
                SetFormData(map[string]string{
                    "agent_id": fmt.Sprintf("%d", agentConfig.AgentID),
                    "display":  fmt.Sprintf("%d", displayIdx),
                }).
                SetFileReader("file", fname, &buf).
                Post(serverURL + "/api/screenshot")
        }(imageBuf, filename, i)
    }
}

func registerSelf() error {
    hostname, _ := os.Hostname()
    localIP := getLocalIP() // Use your existing helper
    
    resp, err := client.R().
        SetBody(map[string]interface{}{
            "hostname":       hostname,
            "os":             runtime.GOOS,
            "ip_address":     localIP,
            "user_full_name": agentConfig.UserFullName,
            "organization":   agentConfig.Organization,
        }).
        Post(serverURL + "/api/register")

    if err != nil {
        return err
    }

    var result map[string]interface{}
    json.Unmarshal(resp.Body(), &result)
    if id, ok := result["agent_id"].(float64); ok {
        agentConfig.AgentID = uint(id)
        return nil
    }
    return fmt.Errorf("invalid response from server")
}

// --- Persistence Helpers ---

func loadConfig() bool {
    file, err := os.ReadFile(configPath)
    if err != nil { return false }
    json.Unmarshal(file, &agentConfig)
    return agentConfig.AgreedToToS
}

func saveConfig() {
    data, _ := json.MarshalIndent(agentConfig, "", "  ")
    os.WriteFile(configPath, data, 0644)
}

func getLocalIP() string {
    addrs, err := net.InterfaceAddrs()
    if err != nil { return "unknown" }
    for _, addr := range addrs {
        if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
            if ipNet.IP.To4() != nil { return ipNet.IP.String() }
        }
    }
    return "unknown"
}
