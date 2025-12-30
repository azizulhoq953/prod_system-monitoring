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

// func (h *MonitorHandler) GetActivityLogs(c *gin.Context) {
//     var activities []models.Activity
    
//     if err := h.DB.Order("timestamp desc").Limit(50).Find(&activities).Error; err != nil {
//         c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
//         return
//     }

//     c.JSON(http.StatusOK, activities)
// }

func (h *MonitorHandler) GetScreenshotGallery(c *gin.Context) {
    type UserGallery struct {
        Username string   `json:"username"`
        Images   []string `json:"images"`
    }

    var gallery []UserGallery
    root := h.UploadDir // e.g., "./uploads"

    // ফোল্ডার রিড করা
    entries, err := os.ReadDir(root)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read uploads folder"})
        return
    }

    for _, entry := range entries {
        if entry.IsDir() {
            username := entry.Name()
            userPath := filepath.Join(root, username)
            
            var images []string
            files, _ := os.ReadDir(userPath)
            
            for _, file := range files {
                // শুধুমাত্র ইমেজ ফাইল নেওয়া
                if strings.HasSuffix(file.Name(), ".png") || strings.HasSuffix(file.Name(), ".jpg") {
                    // URL তৈরি করা: /uploads/Azizul/image.png
                    imgURL := "/uploads/" + username + "/" + file.Name()
                    images = append(images, imgURL)
                }
            }

            // যদি ইমেজ থাকে তবে লিস্টে যোগ করা
            if len(images) > 0 {
                gallery = append(gallery, UserGallery{
                    Username: username,
                    Images:   images,
                })
            }
        }
    }

    c.JSON(http.StatusOK, gallery)
}

//get all agent 
// 1. Get All Agents (For Home Page)
func (h *MonitorHandler) GetAllAgents(c *gin.Context) {
    var agents []models.Agent
    if err := h.DB.Order("last_seen desc").Find(&agents).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agents"})
        return
    }
    c.JSON(http.StatusOK, agents)
}


func (h *MonitorHandler) GetActivityLogs(c *gin.Context) {
    var activities []models.Activity
    agentID := c.Query("agent_id") 

    query := h.DB.Order("timestamp desc").Limit(100)
    
    if agentID != "" {
        query = query.Where("agent_id = ?", agentID)
    }

    if err := query.Find(&activities).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
        return
    }

    c.JSON(http.StatusOK, activities)
}


func (h *MonitorHandler) GetAgentImages(c *gin.Context) {
    agentID := c.Query("agent_id")
    
    if agentID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID required"})
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

    userPath := filepath.Join(h.UploadDir, safeFolderName)
    fmt.Printf("📂 Looking for images in: %s\n", userPath) 

    var images []string
    files, err := os.ReadDir(userPath)
    
    if err == nil {
        for _, file := range files {
            if !file.IsDir() && (strings.HasSuffix(file.Name(), ".png") || strings.HasSuffix(file.Name(), ".jpg")) {
                fullURL := "/uploads/" + safeFolderName + "/" + file.Name()
                images = append(images, fullURL)
            }
        }
    } else {
        fmt.Printf("⚠️ Folder not found: %s\n", userPath)
    }

    for i, j := 0, len(images)-1; i < j; i, j = i+1, j-1 {
        images[i], images[j] = images[j], images[i]
    }

    c.JSON(http.StatusOK, images)
}