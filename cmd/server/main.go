package main

import (
	"fmt"
	"log"
	"monitor-system/internal/handlers"
	"monitor-system/internal/models"
	"os"
	"net/http"
    "time"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func startStatusChecker(db *gorm.DB) {
    ticker := time.NewTicker(30 * time.Second)
    
    go func() {
        for range ticker.C {
            var agents []models.Agent
            db.Find(&agents)
            
            now := time.Now()
            
            for _, agent := range agents {
                timeSinceLastSeen := now.Sub(agent.LastSeen).Seconds()
                
                if timeSinceLastSeen > 60 && agent.Status == "Active" {
                    db.Model(&models.Agent{}).
                        Where("id = ?", agent.ID).
                        Update("status", "Paused")
                    
                    fmt.Printf("Agent %d (%s) marked as Paused (inactive for %.0f seconds)\n", 
                        agent.ID, agent.Hostname, timeSinceLastSeen)
                }
            }
        }
    }()
    
    log.Println("Status checker started (checking every 30 seconds)")
}

func main() {
    err := godotenv.Load()
    if err != nil {
        err = godotenv.Load("../.env")
    }
    if err != nil {
        err = godotenv.Load("../../.env")
    }

    if err != nil {
        log.Println("Warning: .env file not found. Using system environment or defaults.")
    } else {
        log.Println(".env file loaded successfully")
    }

    serverPort := getEnv("SERVER_PORT", ":8080")
    dbName := getEnv("DB_NAME", "central_monitor.db")
    uploadDir := getEnv("UPLOAD_DIR", "./uploads")

    os.MkdirAll(uploadDir, os.ModePerm)
    
    db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
    if err != nil {
        panic("Failed to connect to database")
    }
    db.AutoMigrate(&models.Agent{}, &models.Activity{}, &models.Screenshot{})
    startStatusChecker(db)

    myHandler := handlers.NewMonitorHandler(db, uploadDir)


    r := gin.Default()

    // r.LoadHTMLGlob("*.html") 
	r.LoadHTMLGlob("../../template/*.html")

    r.Static("./../uploads", uploadDir)

    r.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "dashboard.html", nil)
    })

    // API Routes
    r.POST("/api/register", myHandler.RegisterAgent)
    r.POST("/api/activity", myHandler.ReceiveActivity)
    r.POST("/api/screenshot", myHandler.UploadScreenshot)
    r.GET("/api/stats", myHandler.GetDashboardStats)
    r.GET("/api/logs", myHandler.GetActivityLogs)
	r.GET("/api/gallery", myHandler.GetScreenshotGallery)
	r.GET("/api/agents", myHandler.GetAllAgents)         
    r.GET("/api/agent-images", myHandler.GetAgentImages)
    r.GET("/api/activity-by-date", myHandler.GetActivityByDate)
    r.GET("/api/available-dates", myHandler.GetAvailableDates)

    fmt.Printf("Server running on port %s\n", serverPort)
    r.Run(serverPort)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}