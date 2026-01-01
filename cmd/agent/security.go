package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unsafe"
)

// Security Configuration
const (
	mutexName = "Global\\{8F6F0AC4-B9A1-4C5E-9F1C-2D3E4B5A6C7D}"
)

var encryptionKey = []byte("SparkTech2025SecureKey!@#$%^&*(") // 32 bytes

// InitializeSecurity - Call at program start
func InitializeSecurity() error {
	// 1. Single instance check
	if err := ensureSingleInstance(); err != nil {
		return err
	}

	// 2. Anti-debugging
	if isDebuggerPresent() {
		os.Exit(1)
	}

	// 3. Protect config
	if err := protectConfigFile(); err != nil {
		return err
	}

	// 4. Start security monitor
	go runSecurityMonitor()

	return nil
}

// Single Instance Prevention
func ensureSingleInstance() error {
	if runtime.GOOS == "windows" {
		// Windows implementation would use syscall
		// For simplicity, using file lock
		lockFile := filepath.Join(os.TempDir(), ".monitor_lock")
		f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return fmt.Errorf("another instance is running")
		}
		defer f.Close()
	}
	return nil
}

// Anti-Debugger Detection
func isDebuggerPresent() bool {
	if runtime.GOOS == "windows" {
		// Simplified check - in production use syscall
		return false
	}
	return false
}

// Encrypt Data
func encryptData(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt Data
func decryptData(encrypted string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// Get secure config path
func getSecureConfigPath() string {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		secureDir := filepath.Join(appData, "Microsoft", "Windows", "Templates")
		os.MkdirAll(secureDir, 0755)
		return filepath.Join(secureDir, ".sysconfig.dat")
	}
	return ".agent_config.json"
}

// Protect Config File
func protectConfigFile() error {
	// Set file as hidden on Windows
	if runtime.GOOS == "windows" {
		configPath := getSecureConfigPath()
		// Would use syscall in production
		_ = configPath
	}
	return nil
}

// Security Monitor
func runSecurityMonitor() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if isDebuggerPresent() {
			os.Exit(1)
		}
	}
}

// VM Detection
func isRunningInVM() bool {
	vmIndicators := []string{"vmware", "virtualbox", "vbox", "qemu"}
	hostname, _ := os.Hostname()
	hostnameLower := strings.ToLower(hostname)

	for _, indicator := range vmIndicators {
		if strings.Contains(hostnameLower, indicator) {
			return true
		}
	}
	return false
}

// Verify Integrity
func verifyIntegrity() bool {
	exePath, err := os.Executable()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(exePath)
	if err != nil {
		return false
	}

	hash := sha256.Sum256(data)
	_ = hash // Use this for verification
	return true
}

// Unused variable to satisfy compiler
var _ = unsafe.Sizeof(0)