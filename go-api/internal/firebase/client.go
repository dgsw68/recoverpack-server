package firebase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"recoverpack-server/go-api/internal/models"
)

// Client handles database operations and supports both real Firebase and in-memory fallback.
type Client struct {
	isMock          bool
	firestoreClient *firestore.Client
	storageBucket   string

	// In-memory data structures for mock mode
	mu         sync.RWMutex
	projects   map[string]models.Project
	files      map[string][]models.ProjectFile
	evidence   map[string][]models.Evidence
	timelines  map[string][]models.TimelineEvent
	packages   map[string]models.PackageInfo
}

// NewClient initializes a client. If credentials are missing, falls back to in-memory mock storage.
func NewClient() (*Client, error) {
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	credsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")

	// If variables are missing, default to mock fallback
	if projectID == "" || credsJSON == "" {
		log.Println("[FIREBASE] Missing FIREBASE_PROJECT_ID or FIREBASE_CREDENTIALS_JSON. Initializing local in-memory Mock Database.")
		return &Client{
			isMock:    true,
			projects:  make(map[string]models.Project),
			files:     make(map[string][]models.ProjectFile),
			evidence:  make(map[string][]models.Evidence),
			timelines: make(map[string][]models.TimelineEvent),
			packages:  make(map[string]models.PackageInfo),
		}, nil
	}

	log.Printf("[FIREBASE] Attempting to connect to Google Firebase (Project: %s)...", projectID)
	ctx := context.Background()

	conf := &firebase.Config{
		ProjectID:     projectID,
		StorageBucket: bucketName,
	}

	opt := option.WithCredentialsJSON([]byte(credsJSON))
	app, err := firebase.NewApp(ctx, conf, opt)
	if err != nil {
		log.Printf("[FIREBASE] Connection failed: %v. Falling back to local in-memory Mock Database.", err)
		return &Client{
			isMock:    true,
			projects:  make(map[string]models.Project),
			files:     make(map[string][]models.ProjectFile),
			evidence:  make(map[string][]models.Evidence),
			timelines: make(map[string][]models.TimelineEvent),
			packages:  make(map[string]models.PackageInfo),
		}, nil
	}

	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		log.Printf("[FIREBASE] Firestore client creation failed: %v. Falling back to local in-memory Mock Database.", err)
		return &Client{
			isMock:    true,
			projects:  make(map[string]models.Project),
			files:     make(map[string][]models.ProjectFile),
			evidence:  make(map[string][]models.Evidence),
			timelines: make(map[string][]models.TimelineEvent),
			packages:  make(map[string]models.PackageInfo),
		}, nil
	}

	log.Println("[FIREBASE] Connected to Google Firestore successfully.")
	return &Client{
		isMock:          false,
		firestoreClient: firestoreClient,
		storageBucket:   bucketName,
	}, nil
}

// Close closes the firestore connection
func (c *Client) Close() {
	if !c.isMock && c.firestoreClient != nil {
		c.firestoreClient.Close()
	}
}

// IsMock returns whether the client is in-memory fallback mode
func (c *Client) IsMock() bool {
	return c.isMock
}

// --- Project Operations ---

func (c *Client) CreateProject(ctx context.Context, p *models.Project) error {
	p.CreatedAt = time.Now()
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.projects[p.ID] = *p
		return nil
	}

	_, err := c.firestoreClient.Collection("projects").Doc(p.ID).Set(ctx, p)
	return err
}

func (c *Client) GetProject(ctx context.Context, id string) (*models.Project, error) {
	if c.isMock {
		c.mu.RLock()
		defer c.mu.RUnlock()
		p, exists := c.projects[id]
		if !exists {
			return nil, errors.New("project not found")
		}
		return &p, nil
	}

	doc, err := c.firestoreClient.Collection("projects").Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	var p models.Project
	if err := doc.DataTo(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *Client) UpdateProjectDescription(ctx context.Context, id string, desc string) error {
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		p, exists := c.projects[id]
		if !exists {
			return errors.New("project not found")
		}
		p.Description = desc
		c.projects[id] = p
		return nil
	}

	_, err := c.firestoreClient.Collection("projects").Doc(id).Update(ctx, []firestore.Update{
		{Path: "description", Value: desc},
	})
	return err
}

// --- File Operations ---

func (c *Client) CreateFile(ctx context.Context, f *models.ProjectFile) error {
	f.CreatedAt = time.Now()
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.files[f.ProjectID] = append(c.files[f.ProjectID], *f)
		return nil
	}

	_, err := c.firestoreClient.Collection("files").Doc(f.ID).Set(ctx, f)
	return err
}

func (c *Client) GetFilesByProject(ctx context.Context, projectID string) ([]models.ProjectFile, error) {
	if c.isMock {
		c.mu.RLock()
		defer c.mu.RUnlock()
		filesList, exists := c.files[projectID]
		if !exists {
			return []models.ProjectFile{}, nil
		}
		return filesList, nil
	}

	var filesList []models.ProjectFile
	iter := c.firestoreClient.Collection("files").Where("projectId", "==", projectID).Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var f models.ProjectFile
		if err := doc.DataTo(&f); err != nil {
			return nil, err
		}
		filesList = append(filesList, f)
	}
	return filesList, nil
}

// --- Evidence Operations ---

func (c *Client) SaveEvidence(ctx context.Context, evs []models.Evidence) error {
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		if len(evs) == 0 {
			return nil
		}
		projectID := evs[0].ProjectID
		
		// Clear existing evidence for this project before re-saving (or overwrite/append intelligently)
		// For MVP, overwriting the set of evidence on analysis re-run is standard.
		c.evidence[projectID] = evs
		return nil
	}

	// Write batch in Firestore
	batch := c.firestoreClient.Batch()
	for _, ev := range evs {
		docRef := c.firestoreClient.Collection("evidence").Doc(ev.ID)
		batch.Set(docRef, ev)
	}
	_, err := batch.Commit(ctx)
	return err
}

func (c *Client) GetEvidenceByProject(ctx context.Context, projectID string) ([]models.Evidence, error) {
	if c.isMock {
		c.mu.RLock()
		defer c.mu.RUnlock()
		evs, exists := c.evidence[projectID]
		if !exists {
			return []models.Evidence{}, nil
		}
		return evs, nil
	}

	var evs []models.Evidence
	iter := c.firestoreClient.Collection("evidence").Where("projectId", "==", projectID).Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var ev models.Evidence
		if err := doc.DataTo(&ev); err != nil {
			return nil, err
		}
		evs = append(evs, ev)
	}
	return evs, nil
}

func (c *Client) UpdateEvidence(ctx context.Context, projectID string, evidenceID string, category string, caption string) (*models.Evidence, error) {
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		evs, exists := c.evidence[projectID]
		if !exists {
			return nil, errors.New("evidence list not found for project")
		}
		for i, ev := range evs {
			if ev.ID == evidenceID {
				evs[i].Category = category
				evs[i].Caption = caption
				return &evs[i], nil
			}
		}
		return nil, errors.New("evidence item not found")
	}

	docRef := c.firestoreClient.Collection("evidence").Doc(evidenceID)
	doc, err := docRef.Get(ctx)
	if err != nil {
		return nil, err
	}
	var ev models.Evidence
	if err := doc.DataTo(&ev); err != nil {
		return nil, err
	}

	if ev.ProjectID != projectID {
		return nil, fmt.Errorf("evidence does not belong to project %s", projectID)
	}

	ev.Category = category
	ev.Caption = caption

	_, err = docRef.Set(ctx, ev)
	if err != nil {
		return nil, err
	}
	return &ev, nil
}

// --- Timeline Operations ---

func (c *Client) SaveTimelineEvents(ctx context.Context, projectID string, events []models.TimelineEvent) error {
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.timelines[projectID] = events
		return nil
	}

	// Delete old timeline events for project
	iter := c.firestoreClient.Collection("timelines").Where("projectId", "==", projectID).Documents(ctx)
	defer iter.Stop()
	batch := c.firestoreClient.Batch()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	// Save new ones
	for _, ev := range events {
		docRef := c.firestoreClient.Collection("timelines").Doc(ev.ID)
		batch.Set(docRef, ev)
	}

	_, err := batch.Commit(ctx)
	return err
}

func (c *Client) GetTimelineByProject(ctx context.Context, projectID string) ([]models.TimelineEvent, error) {
	if c.isMock {
		c.mu.RLock()
		defer c.mu.RUnlock()
		events, exists := c.timelines[projectID]
		if !exists {
			return []models.TimelineEvent{}, nil
		}
		return events, nil
	}

	var events []models.TimelineEvent
	iter := c.firestoreClient.Collection("timelines").Where("projectId", "==", projectID).Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var ev models.TimelineEvent
		if err := doc.DataTo(&ev); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, nil
}

// --- Package Operations ---

func (c *Client) SavePackage(ctx context.Context, pkg *models.PackageInfo) error {
	pkg.GeneratedAt = time.Now()
	if c.isMock {
		c.mu.Lock()
		defer c.mu.Unlock()
		c.packages[pkg.ProjectID] = *pkg
		return nil
	}

	_, err := c.firestoreClient.Collection("packages").Doc(pkg.ProjectID).Set(ctx, pkg)
	return err
}

func (c *Client) GetPackage(ctx context.Context, projectID string) (*models.PackageInfo, error) {
	if c.isMock {
		c.mu.RLock()
		defer c.mu.RUnlock()
		pkg, exists := c.packages[projectID]
		if !exists {
			return nil, errors.New("package not found")
		}
		return &pkg, nil
	}

	doc, err := c.firestoreClient.Collection("packages").Doc(projectID).Get(ctx)
	if err != nil {
		return nil, err
	}
	var pkg models.PackageInfo
	if err := doc.DataTo(&pkg); err != nil {
		return nil, err
	}
	return &pkg, nil
}
