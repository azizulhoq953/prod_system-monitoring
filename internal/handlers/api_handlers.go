package handlers

import (
	"fmt"
	"monitor-system/internal/models"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

//db struct to hold dependencies
type MonitorHandler struct {
	DB        *gorm.DB
	UploadDir string
}

// constructor function
func NewMonitorHandler(db *gorm.DB, uploadDir string) *MonitorHandler {
	return &MonitorHandler{
		DB:        db,
		UploadDir: uploadDir,
	}
}


// Register Agent
func (h *MonitorHandler) RegisterAgent(c *gin.Context) {
	var input models.Agent
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// h.DB.Where to find existing or create new
	var agent models.Agent
	h.DB.Where(models.Agent{Hostname: input.Hostname}).FirstOrCreate(&agent, models.Agent{Hostname: input.Hostname})
	h.DB.Model(&agent).Updates(models.Agent{OS: input.OS, IPAddress: input.IPAddress, LastSeen: time.Now()})

	c.JSON(http.StatusOK, gin.H{"agent_id": agent.ID})
}

// Log Activity
func (h *MonitorHandler) LogActivity(c *gin.Context) {
	var input models.Activity
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	input.Timestamp = time.Now()
	h.DB.Create(&input)
	c.JSON(http.StatusOK, gin.H{"status": "logged"})
}

// Upload Screenshot
func (h *MonitorHandler) UploadScreenshot(c *gin.Context) {
	agentID := c.PostForm("agent_id")
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Save file to upload directory
	filename := fmt.Sprintf("%s_%d_%s", agentID, time.Now().Unix(), filepath.Base(file.Filename))
	savePath := filepath.Join(h.UploadDir, filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// DB Entry
	var agent models.Agent
	h.DB.First(&agent, agentID)

	h.DB.Create(&models.Screenshot{
		AgentID:   agent.ID,
		FilePath:  savePath,
		Timestamp: time.Now(),
	})

	c.JSON(http.StatusOK, gin.H{"status": "uploaded", "path": savePath})
}