package models

import "time"

// User is a locally authenticated RecoverPack account.
type User struct {
	ID           string    `json:"id" firestore:"id"`
	Email        string    `json:"email" firestore:"email"`
	Name         string    `json:"name" firestore:"name"`
	PasswordHash string    `json:"-" firestore:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt" firestore:"createdAt"`
}

// Project represents a disaster damage project
type Project struct {
	ID          string    `json:"id" firestore:"id"`
	UserID      string    `json:"userId" firestore:"userId"`
	DamageType  string    `json:"damageType" firestore:"damageType"`
	Title       string    `json:"title" firestore:"title"`
	Location    string    `json:"location" firestore:"location"`
	OccurredAt  string    `json:"occurredAt" firestore:"occurredAt"`
	Description string    `json:"description" firestore:"description"`
	CreatedAt   time.Time `json:"createdAt" firestore:"createdAt"`

	// Reporter fields prefill the 피해자 정보 section of the official
	// 자연재난 피해신고서. No resident registration number or bank account
	// is collected here; those stay on the paper form the user fills by hand.
	ReporterName    string `json:"reporterName" firestore:"reporterName"`
	ReporterPhone   string `json:"reporterPhone" firestore:"reporterPhone"`
	ReporterAddress string `json:"reporterAddress" firestore:"reporterAddress"`
	ResidenceType   string `json:"residenceType" firestore:"residenceType"`

	IndirectSupport IndirectSupport `json:"indirectSupport" firestore:"indirectSupport"`
}

// IndirectSupport mirrors the "3. 간접 지원" checklist on the official
// 자연재난 피해신고서, so users can see which extra support to ask about
// when they submit their damage report.
type IndirectSupport struct {
	GasUser                 bool `json:"gasUser" firestore:"gasUser"`
	VehicleOwner            bool `json:"vehicleOwner" firestore:"vehicleOwner"`
	PublicHousingRequest    bool `json:"publicHousingRequest" firestore:"publicHousingRequest"`
	FamilyCrisisSupport     bool `json:"familyCrisisSupport" firestore:"familyCrisisSupport"`
	HealthInsuranceArrears  bool `json:"healthInsuranceArrears" firestore:"healthInsuranceArrears"`
	FineDeferralRequest     bool `json:"fineDeferralRequest" firestore:"fineDeferralRequest"`
	DisasterLossDeduction   bool `json:"disasterLossDeduction" firestore:"disasterLossDeduction"`
	WindFloodInsuranceOptIn bool `json:"windFloodInsuranceOptIn" firestore:"windFloodInsuranceOptIn"`
}

// ProjectFile represents the metadata of an uploaded file
type ProjectFile struct {
	ID          string    `json:"id" firestore:"id"`
	ProjectID   string    `json:"projectId" firestore:"projectId"`
	FileName    string    `json:"fileName" firestore:"fileName"`
	FileType    string    `json:"fileType" firestore:"fileType"`
	FileURL     string    `json:"fileUrl" firestore:"fileUrl"`
	MimeType    string    `json:"mimeType" firestore:"mimeType"`
	Size        int64     `json:"size" firestore:"size"`
	SHA256      string    `json:"sha256" firestore:"sha256"`
	StoragePath string    `json:"-" firestore:"storagePath"`
	CreatedAt   time.Time `json:"createdAt" firestore:"createdAt"`
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
