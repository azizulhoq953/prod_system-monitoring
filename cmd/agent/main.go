package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"image/png"
	"net"
	"os"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/go-resty/resty/v2"
	"github.com/kbinani/screenshot"
	"fyne.io/fyne/v2/data/binding"
)

type AgentConfig struct {
	AgentID      uint   `json:"agent_id"`
	UserFullName string `json:"user_full_name"`
	Organization string `json:"organization"`
	AgreedToToS  bool   `json:"agreed_to_tos"`
}

var (
	client      = resty.New()
	serverURL   = "http://10.10.7.72:8080" 
	agentConfig AgentConfig
	configPath  = "agent_config.json"

	stopTracking  = make(chan bool)
	isTracking    = false
	durationLabel *widget.Label 
    timerData     = binding.NewString()
	startTime     time.Time
	stopTimer     chan bool
	statusLabel   *widget.Label
	previewImage  *canvas.Image
)

func main() {
	a := app.NewWithID("com.sparktech.agent")
	w := a.NewWindow("Workplace Monitor Agent")

	w.Resize(fyne.NewSize(500, 700))
	w.CenterOnScreen()

	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu("Agent",
			fyne.NewMenuItem("Show Dashboard", func() {
				w.Show()
			}),
			fyne.NewMenuItem("Quit Agent", func() {
				a.Quit()
			}),
		)
		desk.SetSystemTrayMenu(m)
	}

	w.SetCloseIntercept(func() {
		w.Hide()
		a.SendNotification(fyne.NewNotification("Agent Hidden", "Running in background..."))
	})

	if !loadConfig() {
		showPrivacyScreen(w, a)
	} else {
		showDashboard(w, a)
	}

	w.ShowAndRun()
}

// --- UI Screens ---

func showPrivacyScreen(w fyne.Window, a fyne.App) {
	w.SetTitle("Workplace Monitor Agent")
	header := widget.NewLabelWithStyle("User Information", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Your Name")

	orgEntry := widget.NewEntry()
	orgEntry.SetPlaceHolder("Organization")

	form := container.NewVBox(
		widget.NewLabel("Your Name:"),
		nameEntry,
		widget.NewLabel("Organization:"),
		orgEntry,
	)

	saveBtn := widget.NewButton("Save & Start", func() {
		if nameEntry.Text == "" || orgEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("Please fill in all fields"), w)
			return
		}

		err := registerSelf(nameEntry.Text, orgEntry.Text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Server connection failed: %v", err), w)
			return
		}

		agentConfig.UserFullName = nameEntry.Text
		agentConfig.Organization = orgEntry.Text
		agentConfig.AgreedToToS = true
		saveConfig()

		showDashboard(w, a)
	})
	saveBtn.Importance = widget.HighImportance

	w.SetContent(container.NewVBox(
		header,
		widget.NewSeparator(),
		form,
		layout.NewSpacer(),
		saveBtn,
	))
}

// Timer Logic (Fixed)
func startTimer() {
    startTime = time.Now()
    stopTimer = make(chan bool)

    go func() {
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-stopTimer:
                return
            case <-ticker.C:
                elapsed := time.Since(startTime)
                h := int(elapsed.Hours())
                m := int(elapsed.Minutes()) % 60
                s := int(elapsed.Seconds()) % 60
                
                timeString := fmt.Sprintf("Active Time: %02d:%02d:%02d", h, m, s)
                // auto update fyne label
                timerData.Set(timeString) 
            }
        }
    }()
}

func showDashboard(w fyne.Window, a fyne.App) {
	// A. Header Section
	headerText := widget.NewLabelWithStyle("Agent Dashboard", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	themeBtn := widget.NewButtonWithIcon("", theme.ColorPaletteIcon(), func() {
		if a.Settings().ThemeVariant() == theme.VariantDark {
			a.Settings().SetTheme(theme.LightTheme())
		} else {
			a.Settings().SetTheme(theme.DarkTheme())
		}
	})

	header := container.NewBorder(nil, nil, headerText, themeBtn)

	// B. Status Card
	infoLabel := widget.NewLabel(fmt.Sprintf("User: %s\nOrg: %s", agentConfig.UserFullName, agentConfig.Organization))
	infoLabel.TextStyle = fyne.TextStyle{Monospace: true}

	statusLabel = widget.NewLabel("Status: IDLE")
	statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	durationLabel = widget.NewLabelWithData(timerData) 
    durationLabel.TextStyle = fyne.TextStyle{Bold: true}

    statusCard := widget.NewCard("System Status", "", container.NewVBox(
        infoLabel,
        widget.NewSeparator(),
        statusLabel,
        durationLabel,
    ))

	previewImage = canvas.NewImageFromResource(theme.FyneLogo())
	previewImage.FillMode = canvas.ImageFillContain
	previewImage.SetMinSize(fyne.NewSize(400, 250))

	bgRect := canvas.NewRectangle(color.NRGBA{R: 40, G: 40, B: 40, A: 50})
	imageContainer := container.NewStack(bgRect, previewImage)

	previewCard := widget.NewCard("Live Preview", "", container.NewPadded(imageContainer))

	// D. Control Buttons (FIXED LOGIC)
	var startBtn *widget.Button
	var stopBtn *widget.Button

	// Start Button Definition
	startBtn = widget.NewButtonWithIcon("START MONITORING", theme.MediaPlayIcon(), func() {
		if isTracking {
			return
		}
		
		// 1. Logic Setup
		isTracking = true
		statusLabel.SetText("Status: MONITORING ACTIVE")
		startBtn.Disable()
		stopBtn.Enable()

		// 2. Start Background Processes
		go runTrackingLoop() // Server tracking
		startTimer()         // UI Timer
	})
	startBtn.Importance = widget.HighImportance

	// Stop Button Definition
	stopBtn = widget.NewButtonWithIcon("STOP MONITORING", theme.MediaStopIcon(), func() {
		if !isTracking {
			return
		}

		// 1. Stop Processes
		stopTracking <- true // Stop server tracking loop
		if stopTimer != nil {
			stopTimer <- true // Stop UI timer
			// close(stopTimer) // Optional: Safe to close if recreated in startTimer
		}

		// 2. Logic Reset
		isTracking = false
		statusLabel.SetText("Status: IDLE")
		startBtn.Enable()
		stopBtn.Disable()
		
	})
	stopBtn.Disable()

	// E. Layout
	content := container.NewVBox(
		header,
		widget.NewSeparator(),
		container.NewPadded(statusCard),
		container.NewPadded(previewCard),
		layout.NewSpacer(),
		container.NewGridWithColumns(2, startBtn, stopBtn),
	)

	w.SetContent(container.NewPadded(content))
}

// --- Background Logic ---

func runTrackingLoop() {
    // check window title every 2 seconds
    activityTicker := time.NewTicker(2 * time.Second)
    // every 30 second server heartbeat
    heartbeatTicker := time.NewTicker(30 * time.Second) 
    
    screenshotTicker := time.NewTicker(60 * time.Second)

    defer activityTicker.Stop()
    defer heartbeatTicker.Stop()
    defer screenshotTicker.Stop()

    lastWindow := ""


	//helper function to send activity
sendActivity := func(windowName string) {
  fmt.Println("Sending heartbeat...", time.Now().Format("15:04:05"))

    resp, err := client.R().
        SetBody(map[string]interface{}{
            "agent_id":  agentConfig.AgentID,
            "window":    windowName,
            "timestamp": time.Now(),
        }).
        Post(serverURL + "/api/activity")

        if err != nil {
            fmt.Println("❌ Error sending heartbeat:", err)
        } else {
            fmt.Printf("✅ Server Responded: Status: %d, Body: %s\n", resp.StatusCode(), resp.String())
        }
    }

    for {
        select {
        case <-stopTracking:
            return

        case <-activityTicker.C:
            currentWindow := getActiveWindowTitle()
            if currentWindow != "" && currentWindow != lastWindow {
                sendActivity(currentWindow)
                lastWindow = currentWindow
            }

        case <-heartbeatTicker.C:

            if lastWindow != "" {
                sendActivity(lastWindow)
            } else {
                 sendActivity("System Idle") 
            }

        case <-screenshotTicker.C:
            captureAndUpload()
        }
    }
}

func captureAndUpload() {
    n := screenshot.NumActiveDisplays()
    if n <= 0 {
        return
    }

    for i := 0; i < n; i++ {
        bounds := screenshot.GetDisplayBounds(i)
        img, err := screenshot.CaptureRect(bounds)
        if err != nil {
            continue
        }

        var imageBuf bytes.Buffer
        png.Encode(&imageBuf, img)

        // UI update fix (Thread Safe)
        if i == 0 && previewImage != nil {
            // We take the image into a local variable for safety
            currentImg := img
            
            // Using fyne.Do to send update to main thread
            fyne.Do(func() {
                previewImage.Image = currentImg
                previewImage.Refresh()  //solve issues 336 
            })
        }

        filename := fmt.Sprintf("screen_%d_%d.png", agentConfig.AgentID, i)

        // Upload Logic (Background Goroutine)
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

func registerSelf(name, org string) error {
	hostname, _ := os.Hostname()
	localIP := getLocalIP()

	resp, err := client.R().
		SetBody(map[string]interface{}{
			"hostname":       hostname,
			"os":             runtime.GOOS,
			"ip_address":     localIP,
			"user_full_name": name,
			"organization":   org,
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

func loadConfig() bool {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	json.Unmarshal(file, &agentConfig)
	return agentConfig.AgreedToToS
}

func saveConfig() {
	data, _ := json.MarshalIndent(agentConfig, "", "  ")
	os.WriteFile(configPath, data, 0644)
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
        }
    }
    return "unknown"
}