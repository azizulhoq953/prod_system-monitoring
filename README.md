# 🕵️ System Monitoring Agent

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux-lightgrey)
![License](https://img.shields.io/badge/License-MIT-blue)

A lightweight, cross-platform **Employee Monitoring System** built with Golang. This system consists of a central **Backend Server** and a stealth **Agent** that runs on client machines to track activity and capture screenshots.

## ✨ Features

- **🚀 Auto-Registration:** Agents automatically register themselves with the server upon startup.
- **🖥️ Active Window Tracking:** Logs the title of the currently active window in real-time.
- **📸 Automated Screenshots:** Captures screen content at configurable intervals.
- **👻 Stealth Mode:** Can run in the background without a visible console window (Windows).
- **📂 Centralized Storage:** All logs and images are stored securely on the server.

---

## 🛠️ Configuration (.env)

Create a `.env` file in the root directory of the project.

```ini
# Server Configuration
SERVER_PORT=:8080
SERVER_URL=[http://10.10.7.72:8080](http://10.10.7.72:8080)  # Replace with your Server IP

# Agent Configuration
SCREENSHOT_INTERVAL_SEC=60         # Screenshot frequency in seconds

# Database & Storage
DB_NAME=central_monitor.db
UPLOAD_DIR=./uploads

# App Mode
APP_MODE=production# prod_system-monitoring
