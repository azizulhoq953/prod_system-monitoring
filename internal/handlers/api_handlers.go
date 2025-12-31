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

type AgentResponse struct {
    models.Agent
    Status     string `json:"status"`      
    ActiveTime string `json:"active_time"` 
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



func (h *MonitorHandler) GetScreenshotGallery(c *gin.Context) {
    type UserGallery struct {
        Username string   `json:"username"`
        Images   []string `json:"images"`
    }

    var gallery []UserGallery
    root := h.UploadDir

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
                if strings.HasSuffix(file.Name(), ".png") || strings.HasSuffix(file.Name(), ".jpg") {
                    imgURL := "/uploads/" + username + "/" + file.Name()
                    images = append(images, imgURL)
                }
            }

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


func (h *MonitorHandler) GetAllAgents(c *gin.Context) {
    var agents []models.Agent
    
    // loaded all agents from DB
    if err := h.DB.Order("last_seen desc").Find(&agents).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agents"})
        return
    }

    var responseList []AgentResponse
    
    // Today's date (from midnight) reset time
    startOfDay := time.Now().Truncate(24 * time.Hour)

    for _, agent := range agents {
        // If seen within last 2 minutes, then Active, else Paused
        status := "Paused"
        if time.Since(agent.LastSeen) < 5*time.Minute {
            status = "Active"
        }

        //  Active Time Logic (Daily Reset) ---
        // count today's activity logs for this agent
        var activityCount int64
        h.DB.Model(&models.Activity{}).
            Where("agent_id = ? AND timestamp >= ?", agent.ID, startOfDay).
            Count(&activityCount)

        // Active Time Calculation ---
        // if 1 log = 1 minute active time
        // law of total minutes
        totalMinutes := int(activityCount) // if reccived 1 log = 1 minute
        
        hours := totalMinutes / 60
        minutes := totalMinutes % 60
        timeString := fmt.Sprintf("%dh %dm", hours, minutes)

        // append to response list
        responseList = append(responseList, AgentResponse{
            Agent:      agent,
            Status:     status,
            ActiveTime: timeString,
        })
    }

    c.JSON(http.StatusOK, responseList)
}

func (h *MonitorHandler) GetActivityLogs(c *gin.Context) {
    var activities []models.Activity
    agentID := c.Query("agent_id") 
    date := c.Query("date")     
    month := c.Query("month")

    query := h.DB.Order("timestamp desc").Limit(100)
    
    if agentID != "" {
        query = query.Where("agent_id = ?", agentID)
    }
    if date != "" {
        
        startOfDay, err := time.Parse("2006-01-02", date)
        if err == nil {
            endOfDay := startOfDay.Add(24 * time.Hour)
            query = query.Where("timestamp >= ? AND timestamp < ?", startOfDay, endOfDay)
            fmt.Printf("📅 Filtering logs for date: %s\n", date)
        }
    } else if month != "" {
      
        startOfMonth, err := time.Parse("2006-01", month)
        if err == nil {
            endOfMonth := startOfMonth.AddDate(0, 1, 0)
            query = query.Where("timestamp >= ? AND timestamp < ?", startOfMonth, endOfMonth)
            fmt.Printf("📅 Filtering logs for month: %s\n", month)
        }
    }
    if err := query.Find(&activities).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch logs"})
        return
    }

    c.JSON(http.StatusOK, activities)
}


func (h *MonitorHandler) GetActivityByDate(c *gin.Context) {
    agentID := c.Query("agent_id")
    date := c.Query("date")
    month := c.Query("month") 
    
    if agentID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID required"})
        return
    }

    var activities []models.Activity
    query := h.DB.Where("agent_id = ?", agentID).Order("timestamp desc")

    if date != "" {
        // Specific day
        startOfDay, _ := time.Parse("2006-01-02", date)
        endOfDay := startOfDay.Add(24 * time.Hour)
        query = query.Where("timestamp >= ? AND timestamp < ?", startOfDay, endOfDay)
    } else if month != "" {
        // Specific month
        startOfMonth, _ := time.Parse("2006-01", month)
        endOfMonth := startOfMonth.AddDate(0, 1, 0)
        query = query.Where("timestamp >= ? AND timestamp < ?", startOfMonth, endOfMonth)
    }

    if err := query.Find(&activities).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch activities"})
        return
    }

    c.JSON(http.StatusOK, activities)
}

// Update GetAgentImages to support date filtering
func (h *MonitorHandler) GetAgentImages(c *gin.Context) {
    agentID := c.Query("agent_id")
    date := c.Query("date")
    month := c.Query("month")
    
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

    var screenshots []models.Screenshot
    query := h.DB.Where("agent_id = ?", agentID).Order("timestamp desc")

    if date != "" {
        startOfDay, _ := time.Parse("2006-01-02", date)
        endOfDay := startOfDay.Add(24 * time.Hour)
        query = query.Where("timestamp >= ? AND timestamp < ?", startOfDay, endOfDay)
    } else if month != "" {
        startOfMonth, _ := time.Parse("2006-01", month)
        endOfMonth := startOfMonth.AddDate(0, 1, 0)
        query = query.Where("timestamp >= ? AND timestamp < ?", startOfMonth, endOfMonth)
    }

    query.Find(&screenshots)

    var images []string
    for _, screenshot := range screenshots {
        filename := filepath.Base(screenshot.FilePath)
        fullURL := "/uploads/" + safeFolderName + "/" + filename
        
        // Verify file exists
        if _, err := os.Stat(screenshot.FilePath); err == nil {
            images = append(images, fullURL)
        }
    }

    c.JSON(http.StatusOK, images)
}

// Add this to get available dates for an agent
func (h *MonitorHandler) GetAvailableDates(c *gin.Context) {
    agentID := c.Query("agent_id")
    
    if agentID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID required"})
        return
    }

    type DateCount struct {
        Date  string `json:"date"`
        Count int64  `json:"count"`
    }

    var results []DateCount
    h.DB.Model(&models.Activity{}).
        Select("DATE(timestamp) as date, COUNT(*) as count").
        Where("agent_id = ?", agentID).
        Group("DATE(timestamp)").
        Order("date desc").
        Scan(&results)

    c.JSON(http.StatusOK, results)
}

//reccived activity purposes dashboard status update

func (h *MonitorHandler) ReceiveActivity(c *gin.Context) {
    var activity models.Activity

    // . JSON Binding
    if err := c.ShouldBindJSON(&activity); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }


    // ৩. save activity to DB
    if err := h.DB.Create(&activity).Error; err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save activity"})
        return
    }

    // update agent's last seen and status to Active
    result := h.DB.Model(&models.Agent{}).
        Where("id = ?", activity.AgentID).
        Updates(map[string]interface{}{
            "last_seen": time.Now(),
            "status":    "Active",
        })

    // check for errors and rows affected
    if result.Error != nil {
        fmt.Printf("❌ DB Update Error for Agent %d: %v\n", activity.AgentID, result.Error)
    } else if result.RowsAffected == 0 {
        fmt.Printf("⚠️ WARNING: Tried to update Agent %d but NO ROWS were affected! (ID mismatch?)\n", activity.AgentID)
    } else {
        fmt.Printf("✅ Success: Updated Agent %d status to Active.\n", activity.AgentID)
    }

    c.JSON(http.StatusOK, gin.H{"status": "logged"})
}