package main

import (
	"fmt"
	"log"
	"monitor-system/internal/handlers"
	"monitor-system/internal/models"
	"os"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// func main() {
// err := godotenv.Load()
//     if err != nil {
//         err = godotenv.Load("../.env")
//     }
//     if err != nil {
//         err = godotenv.Load("../../.env")
//     }

//     // Final check
//     if err != nil {
//         log.Println("Warning: .env file not found. Using system environment or defaults.")
//     } else {
//         log.Println(".env file loaded successfully")
//     }

//     serverPort := getEnv("SERVER_PORT", ":8080")
// 	dbName := getEnv("DB_NAME", "central_monitor.db")
// 	uploadDir := getEnv("UPLOAD_DIR", "./uploads")

// 	// setup folder and database
// 	os.MkdirAll(uploadDir, os.ModePerm)
	
// 	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
// 	if err != nil {
// 		panic("Failed to connect to database")
// 	}
// 	// run migrations
// 	db.AutoMigrate(&models.Agent{}, &models.Activity{}, &models.Screenshot{})

// 	myHandler := handlers.NewMonitorHandler(db, uploadDir)

// 	// router setup
// 	r := gin.Default()
// 	r.Static("/uploads", uploadDir)

// 	// now we use myHandler's methods as handlers
// 	r.POST("/api/register", myHandler.RegisterAgent)
// 	r.POST("/api/activity", myHandler.LogActivity)
// 	r.POST("/api/screenshot", myHandler.UploadScreenshot)
// 	r.GET("/api/stats", myHandler.GetDashboardStats)
// 	r.GET("/api/logs", myHandler.GetActivityLogs)

// 	fmt.Printf("Server running on port %s\n", serverPort)
// 	r.Run(serverPort)
// }

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

    myHandler := handlers.NewMonitorHandler(db, uploadDir)


    r := gin.Default()

    r.LoadHTMLGlob("*.html") 

    r.Static("./../uploads", uploadDir)

    r.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "dashboard.html", nil)
    })

    // API Routes
    r.POST("/api/register", myHandler.RegisterAgent)
    r.POST("/api/activity", myHandler.LogActivity)
    r.POST("/api/screenshot", myHandler.UploadScreenshot)
    r.GET("/api/stats", myHandler.GetDashboardStats)
    r.GET("/api/logs", myHandler.GetActivityLogs)
	r.GET("/api/gallery", myHandler.GetScreenshotGallery)
	r.GET("/api/agents", myHandler.GetAllAgents)         
    r.GET("/api/agent-images", myHandler.GetAgentImages)

    fmt.Printf("Server running on port %s\n", serverPort)
    r.Run(serverPort)
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}