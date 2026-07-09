package models

import "time"

// Project represents a disaster damage project
type Project struct {
	ID          string    `json:"id" firestore:"id"`
	DamageType  string    `json:"damageType" firestore:"damageType"`
	Title       string    `json:"title" firestore:"title"`
	Location    string    `json:"location" firestore:"location"`
	OccurredAt  string    `json:"occurredAt" firestore:"occurredAt"`
	Description string    `json:"description" firestore:"description"`
	CreatedAt   time.Time `json:"createdAt" firestore:"createdAt"`
}

// ProjectFile represents the metadata of an uploaded file
type ProjectFile struct {
	ID        string    `json:"id" firestore:"id"`
	ProjectID string    `json:"projectId" firestore:"projectId"`
	FileName  string    `json:"fileName" firestore:"fileName"`
	FileType  string    `json:"fileType" firestore:"fileType"`
	FileURL   string    `json:"fileUrl" firestore:"fileUrl"`
	MimeType  string    `json:"mimeType" firestore:"mimeType"`
	CreatedAt time.Time `json:"createdAt" firestore:"createdAt"`
}

// Evidence represents the AI-analyzed damage classification and description
type Evidence struct {
	ID        string    `json:"id" firestore:"id"`
	ProjectID string    `json:"projectId" firestore:"projectId"`
	FileID    string    `json:"fileId" firestore:"fileId"`
	FileURL   string    `json:"fileUrl" firestore:"fileUrl"`
	Category  string    `json:"category" firestore:"category"`
	Caption   string    `json:"caption" firestore:"caption"`
	CreatedAt time.Time `json:"createdAt" firestore:"createdAt"`
}

// TimelineEvent represents a single event in the disaster timeline
type TimelineEvent struct {
	ID          string    `json:"id" firestore:"id"`
	ProjectID   string    `json:"projectId" firestore:"projectId"`
	Title       string    `json:"title" firestore:"title"`
	Description string    `json:"description" firestore:"description"`
	EventDate   string    `json:"eventDate" firestore:"eventDate"`
	CreatedAt   time.Time `json:"createdAt" firestore:"createdAt"`
}

// PackageInfo contains the mock or real PDF/ZIP report bundle download URL
type PackageInfo struct {
	ProjectID   string    `json:"projectId" firestore:"projectId"`
	PackageURL  string    `json:"packageUrl" firestore:"packageUrl"`
	Contents    []string  `json:"contents" firestore:"contents"`
	GeneratedAt time.Time `json:"generatedAt" firestore:"generatedAt"`
}
