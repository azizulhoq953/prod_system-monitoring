package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"monitor-system/internal/handlers"
	"monitor-system/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using system defaults.")
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

	myHandler := handlers.NewMonitorHandler(db, uploadDir)

	r := gin.Default()

    r.Static("/assets", "../../assets") 
    // r.Static("/uploads", "../../uploads")
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
	r.PATCH("/api/update-agent", myHandler.UpdateAgentHostname)

	fmt.Printf("Server running on port %s\n", serverPort)
	r.Run(serverPort)
}