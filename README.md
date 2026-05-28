# FESApp Tracker

<p align="center">
  <img src="frontend/src/assets/images/first-encoding_logo.webp" alt="FESApp Tracker Logo" width="200" />
</p>

<p align="center">
  <strong>Employee Task & Attendance Tracking Application</strong>
</p>

<p align="center">
  <a href="https://github.com/FES-Paul/FES-APP-TRACKER/releases">
    <img src="https://img.shields.io/github/v/release/FES-Paul/FES-APP-TRACKER?include_prereleases&label=version" alt="Version" />
  </a>
  <img src="https://img.shields.io/github/license/FES-Paul/FES-APP-TRACKER" alt="License" />
  <img src="https://img.shields.io/github/last-commit/FES-Paul/FES-APP-TRACKER" alt="Last Commit" />
</p>

---

## 📋 Table of Contents

- [About](#about)
- [Features](#features)
- [Technology Stack](#technology-stack)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Development](#development)
  - [Building](#building)
- [Database](#database)
- [Configuration](#configuration)
- [License](#license)

---

## About

**FESApp Tracker** is a desktop application developed by **First Encoding Services** for employee task and attendance tracking. It allows employees to track their work tasks, capture screenshots during work sessions, and sync their attendance data to the cloud.

**Version:** 2.1.0  
**Author:** Paul (johnpaultunacao.firstencoding@gmail.com)

---

## ✨ Features

### Core Features

| Feature                 | Description                                              |
| ----------------------- | -------------------------------------------------------- |
| **Task Tracking**       | Start/stop timer for work tasks with real-time tracking  |
| **Screenshot Capture**  | Automatic periodic screenshots during task work sessions |
| **Attendance Tracking** | Automatic login/logout attendance recording              |
| **Activity Logs**       | View complete work history with date filtering           |
| **Client Management**   | Manage multiple clients and their associated tasks       |

### Connectivity

| Feature           | Description                                      |
| ----------------- | ------------------------------------------------ |
| **Offline Mode**  | Work offline with local SQLite database storage  |
| **Online Mode**   | Sync with Firebase for cloud authentication      |
| **Cloud Storage** | Upload screenshots to Backblaze B2 cloud storage |
| **Auto Sync**     | Automatic synchronization of unsynced records    |

### Desktop Integration

| Feature             | Description                                    |
| ------------------- | ---------------------------------------------- |
| **System Tray**     | Runs in background with system tray icon       |
| **Auto Update**     | Automatic update checking from GitHub releases |
| **Multi-Monitor**   | Screenshot capture from multiple monitors      |
| **Single Instance** | Prevents multiple instances of the app         |

### User Features

| Feature                 | Description                                    |
| ----------------------- | ---------------------------------------------- |
| **User Authentication** | Firebase Google authentication                 |
| **Profile Management**  | View and update user profile information       |
| **Dashboard**           | Overview of tasks, time spent, and sync status |
| **Search**              | Search and filter tasks                        |

---

## 🛠 Technology Stack

### Backend

| Technology | Purpose                            |
| ---------- | ---------------------------------- |
| **Go**     | Core application runtime           |
| **Wails**  | Desktop app framework              |
| **GORM**   | SQLite ORM for database operations |
| **SQLite** | Local database storage             |

### Frontend

| Technology       | Purpose                     |
| ---------------- | --------------------------- |
| **React**        | UI framework                |
| **TypeScript**   | Type-safe JavaScript        |
| **Vite**         | Build tool and dev server   |
| **Tailwind CSS** | Utility-first CSS framework |
| **React Router** | Client-side routing         |
| **Lucide React** | Icon library                |
| **Radix UI**     | UI component primitives     |

### Integrations

| Technology       | Purpose                   |
| ---------------- | ------------------------- |
| **Firebase**     | Cloud authentication      |
| **Backblaze B2** | Cloud screenshot storage  |
| **Upwork**       | External task integration |

---

## 🚀 Getting Started

### Prerequisites

- **Go** 1.20 or higher
- **Node.js** 18 or higher
- **npm** 9 or higher
- **Wails** CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/FES-Paul/FES-APP-TRACKER.git
cd FES-APP-TRACKER
```

2. Install Go dependencies:

```bash
go mod download
```

3. Install frontend dependencies:

```bash
cd frontend
npm install
cd ..
```

### Development

To run in live development mode:

```bash
wails dev
```

This will start:

- A Vite development server with hot reload for the frontend
- A dev server at http://localhost:34115 for accessing Go methods from the browser

### Building

To build a redistributable production package:

```bash
wails build
```

This creates:

- Windows installer: `build/windows/FESApp Tracker Setup.exe`
- Standalone executable: `build/FESApp Tracker.exe`

The built application will be in the `build/` directory.

---

## 🗄️ Database

### Storage Location

The SQLite database is stored in the user's config directory:

- **Windows:** `%APPDATA%/wails-app/`
- **Dev Mode:** `fesapp-dev.db`
- **Production:** `fesapp.db`

### Database Schema

#### Users Table

| Field      | Type   | Description                |
| ---------- | ------ | -------------------------- |
| ID         | string | Primary key (Firebase UID) |
| Name       | string | Full name                  |
| FirstName  | string | First name                 |
| LastName   | string | Last name                  |
| MiddleName | string | Middle name                |
| Position   | string | Job position               |
| Birthdate  | string | Date of birth              |
| Phone      | string | Phone number               |
| Address    | string | Home address               |
| Email      | string | Email address              |
| FESID      | string | Company employee ID        |
| Photo      | string | Profile photo URL          |

#### Tasks Table

| Field       | Type   | Description                   |
| ----------- | ------ | ----------------------------- |
| ID          | uint   | Primary key                   |
| TaskID      | string | Unique task identifier (UUID) |
| Title       | string | Task title                    |
| ProjectName | string | Project name                  |
| ClientID    | string | Foreign key to Client         |

#### TaskRecords Table

| Field       | Type         | Description                  |
| ----------- | ------------ | ---------------------------- |
| ID          | uint         | Primary key                  |
| TaskID      | string       | Foreign key to Task          |
| UserID      | string       | Foreign key to User          |
| StartedAt   | timestamp    | Task start time              |
| EndedAt     | timestamp    | Task end time                |
| Duration    | uint32       | Duration in seconds          |
| Screenshots | []Screenshot | Related screenshots          |
| Notes       | string       | Task notes                   |
| SyncedAt    | timestamp    | Last sync timestamp          |
| SyncStatus  | string       | Sync status (pending/synced) |

#### Screenshots Table

| Field        | Type      | Description               |
| ------------ | --------- | ------------------------- |
| ID           | uint      | Primary key               |
| TaskRecordID | uint      | Foreign key to TaskRecord |
| Filename     | string    | Screenshot filename       |
| FolderName   | string    | Screenshot folder         |
| Path         | string    | Local file path           |
| SyncedAt     | timestamp | Last sync timestamp       |
| SyncStatus   | string    | Sync status               |

#### Attendance Table

| Field         | Type      | Description            |
| ------------- | --------- | ---------------------- |
| ID            | uint      | Primary key            |
| UserID        | string    | Foreign key to User    |
| ClientTime    | uint32    | Total client work time |
| TotalGap      | uint32    | Total gap time         |
| TimeIn        | timestamp | Login time             |
| TimeOut       | timestamp | Logout time            |
| LoginLogoutID | string    | Session identifier     |
| SyncedAt      | timestamp | Last sync timestamp    |
| SyncStatus    | string    | Sync status            |

### Migrations

Database migrations are run automatically on app startup using GORM's AutoMigrate feature.

---

## ⚙️ Configuration

### App Configuration

The main configuration file is `wails.json`:

```json
{
  "name": "fesapp",
  "outputfilename": "FesApp Tracker",
  "frontend:install": "npm install",
  "frontend:build": "npm run build",
  "info": {
    "productName": "FESApp Tracker",
    "productVersion": "2.1.0",
    "companyName": "First Encoding Services",
    "copyright": "Copyright 2025 First Encoding Services"
  }
}
```

### Environment Variables

| Variable | Description      | Default     |
| -------- | ---------------- | ----------- |
| `GO_ENV` | Environment mode | development |

### Key Storage

Credentials are stored securely using the system keyring:

- Firebase refresh tokens
- Cloud storage credentials

---

## 📁 Project Structure

```
FES-APP-TRACKER/
├── backend/
│   ├── app/              # Wails app configuration
│   ├── config.go         # App configuration
│   ├── db/               # Database layer
│   │   ├── db.go         # Database initialization
│   │   └── models/       # Database models
│   ├── repository/      # Data access layer
│   ├── service_classes/  # Business logic services
│   ├── services/         # Additional services
│   ├── types/           # Type definitions
│   └── utils/            # Utility functions
├── frontend/
│   ├── src/
│   │   ├── components/  # React components
│   │   ├── hooks/       # Custom React hooks
│   │   ├── layouts/     # Page layouts
│   │   ├── lib/         # Utility libraries
│   │   ├── pages/       # Page components
│   │   ├── store/       # State management
│   │   └── styles/      # CSS styles
│   ├── index.html       # HTML entry point
│   ├── package.json    # Frontend dependencies
│   └── vite.config.ts   # Vite configuration
├── main.go              # Go entry point
├── wails.json          # Wails configuration
└── README.md           # This file
```

---

## 📄 License

Copyright © 2025 **First Encoding Services**. All rights reserved.

---

## 🆘 Support

For support and bug reports, please contact:

- Email: johnpaultunacao.firstencoding@gmail.com
- GitHub Issues: https://github.com/FES-Paul/FES-APP-TRACKER/issues
