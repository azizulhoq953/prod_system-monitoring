package handlers

import (
    "fmt"
    "monitor-system/internal/models"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)

type MonitorHandler struct {
    DB        *gorm.DB
    UploadDir string
}

func NewMonitorHandler(db *gorm.DB, uploadDir string) *MonitorHandler {
    return &MonitorHandler{
        DB:        db,
        UploadDir: uploadDir,
    }
}

func (h *MonitorHandler) RegisterAgent(c *gin.Context) {
    var input models.Agent
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var agent models.Agent
    h.DB.Where(models.Agent{Hostname: input.Hostname}).FirstOrCreate(&agent, models.Agent{Hostname: input.Hostname})

    h.DB.Model(&agent).Updates(models.Agent{
        UserFullName: input.UserFullName,
        Organization: input.Organization,
        OS:           input.OS,
        IPAddress:    input.IPAddress,
        LastSeen:     time.Now(),
    })

    c.JSON(http.StatusOK, gin.H{"agent_id": agent.ID})
}

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

func (h *MonitorHandler) UploadScreenshot(c *gin.Context) {
    agentID := c.PostForm("agent_id")
    file, err := c.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
        return
    }

    var agent models.Agent
    if err := h.DB.First(&agent, agentID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
        return
    }

    folderName := agent.UserFullName
    if folderName == "" {
        folderName = agent.Hostname 
    }
    safeFolderName := strings.ReplaceAll(folderName, " ", "_")

    userDir := filepath.Join(h.UploadDir, safeFolderName)
    if err := os.MkdirAll(userDir, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user directory"})
        return
    }

    filename := fmt.Sprintf("%d_%d_%s", agent.ID, time.Now().Unix(), filepath.Base(file.Filename))
    savePath := filepath.Join(userDir, filename)

    if err := c.SaveUploadedFile(file, savePath); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
        return
    }

    h.DB.Create(&models.Screenshot{
        AgentID:   agent.ID,
        FilePath:  savePath, 
        Timestamp: time.Now(),
    })

    c.JSON(http.StatusOK, gin.H{"status": "uploaded", "path": savePath})
}

func (h *MonitorHandler) GetDashboardStats(c *gin.Context) {
    var totalUsers int64
    var totalScreenshots int64

    h.DB.Model(&models.Agent{}).Count(&totalUsers)
    

    h.DB.Model(&models.Screenshot{}).Count(&totalScreenshots)

    c.JSON(http.StatusOK, gin.H{
        "total_users":       totalUsers,
        "total_screenshots": totalScreenshots,
    })
}