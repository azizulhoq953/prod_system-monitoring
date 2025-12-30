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

	stopTracking = make(chan bool)
	isTracking   = false

	statusLabel  *widget.Label
	previewImage *canvas.Image
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

	// 4. Load Logic
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

		// 2. Save config locally
		agentConfig.UserFullName = nameEntry.Text
		agentConfig.Organization = orgEntry.Text
		agentConfig.AgreedToToS = true
		saveConfig()

		// 3. Go to Dashboard
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

	statusCard := widget.NewCard("System Status", "", container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		statusLabel,
	))

	// C. Preview Section
	previewImage = canvas.NewImageFromResource(theme.FyneLogo())
	previewImage.FillMode = canvas.ImageFillContain
	previewImage.SetMinSize(fyne.NewSize(400, 250))

	bgRect := canvas.NewRectangle(color.NRGBA{R: 40, G: 40, B: 40, A: 50})
	imageContainer := container.NewStack(bgRect, previewImage)

	previewCard := widget.NewCard("Live Preview", "", container.NewPadded(imageContainer))

	// D. Control Buttons
	var startBtn *widget.Button
	var stopBtn *widget.Button

	startBtn = widget.NewButtonWithIcon("START MONITORING", theme.MediaPlayIcon(), func() {
		if isTracking {
			return
		}
		isTracking = true
		statusLabel.SetText("Status: MONITORING ACTIVE")
		startBtn.Disable()
		stopBtn.Enable()
		go runTrackingLoop()
	})
	startBtn.Importance = widget.HighImportance

	stopBtn = widget.NewButtonWithIcon("STOP MONITORING", theme.MediaStopIcon(), func() {
		if !isTracking {
			return
		}
		stopTracking <- true
		isTracking = false
		statusLabel.SetText("Status: PAUSED")
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
	activityTicker := time.NewTicker(2 * time.Second) // Slowed down slightly
	screenshotTicker := time.NewTicker(60 * time.Second)

	defer activityTicker.Stop()
	defer screenshotTicker.Stop()

	lastWindow := ""

	for {
		select {
		case <-stopTracking:
			return

		case <-activityTicker.C:
			currentWindow := getActiveWindowTitle()
			if currentWindow != "" && currentWindow != lastWindow {
				client.R().
					SetBody(map[string]interface{}{
						"agent_id":  agentConfig.AgentID,
						"window":    currentWindow,
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

		// UI Update
		if i == 0 && previewImage != nil {
			previewImage.Image = img
			previewImage.Refresh()
		}

		filename := fmt.Sprintf("screen_%d_%d.png", agentConfig.AgentID, i)

		// Upload
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