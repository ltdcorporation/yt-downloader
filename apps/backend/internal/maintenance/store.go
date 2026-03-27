package maintenance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"yt-downloader/backend/internal/config"
)

var (
	ErrInvalidInput    = errors.New("invalid maintenance input")
	ErrVersionConflict = errors.New("maintenance version conflict")
)

const (
	defaultEstimatedDowntime = "45 minutes"
	defaultPublicMessage     = "Our systems are currently undergoing a scheduled core infrastructure update. We expect to be back online shortly. Thank you for your patience."
)

type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	if strings.TrimSpace(e.Message) == "" {
		return "invalid maintenance input"
	}
	return e.Message
}

type ServiceKey string

const (
	ServiceAPIGateway      ServiceKey = "api_gateway"
	ServicePrimaryDatabase ServiceKey = "primary_database"
	ServiceWorkerNodes     ServiceKey = "worker_nodes"
)

type ServiceStatus string

const (
	StatusActive      ServiceStatus = "active"
	StatusMaintenance ServiceStatus = "maintenance"
	StatusScaling     ServiceStatus = "scaling"
	StatusRefreshing  ServiceStatus = "refreshing"
)

type ServiceOverride struct {
	Key     ServiceKey
	Name    string
	Status  ServiceStatus
	Enabled bool
}

type Data struct {
	Enabled           bool
	EstimatedDowntime string
	PublicMessage     string
	Services          []ServiceOverride
}

type Snapshot struct {
	Data            Data
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	UpdatedByUserID string
}

type ServicePatch struct {
	Key     ServiceKey
	Status  *ServiceStatus
	Enabled *bool
}

type Patch struct {
	Enabled           *bool
	EstimatedDowntime *string
	PublicMessage     *string
	Services          []ServicePatch
}

type ApplyPatchParams struct {
	Before        Snapshot
	After         Snapshot
	ChangedFields []string
	ActorUserID   string
	RequestID     string
	Source        string
	ChangedAt     time.Time
	AuditID       string
}

type backend interface {
	Close() error
	EnsureReady(ctx context.Context) error
	GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error)
	ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error)
}

type Store struct {
	backend backend
}

func NewStore(cfg config.Config, logger *log.Logger) *Store {
	if strings.TrimSpace(cfg.PostgresDSN) != "" {
		if logger != nil {
			logger.Printf("maintenance store engine=postgres")
		}
		return &Store{backend: newPostgresBackend(cfg.PostgresDSN)}
	}

	if logger != nil {
		logger.Printf("maintenance store engine=memory (POSTGRES_DSN empty)")
	}
	return &Store{backend: newMemoryBackend()}
}

func (s *Store) Close() error {
	if s == nil || s.backend == nil {
		return nil
	}
	return s.backend.Close()
}

func (s *Store) EnsureReady(ctx context.Context) error {
	if s == nil || s.backend == nil {
		return errors.New("maintenance store is not initialized")
	}
	return s.backend.EnsureReady(ctx)
}

func (s *Store) GetOrCreateSnapshot(ctx context.Context, now time.Time) (Snapshot, error) {
	if s == nil || s.backend == nil {
		return Snapshot{}, errors.New("maintenance store is not initialized")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	snapshot, err := s.backend.GetOrCreateSnapshot(ctx, now.UTC())
	if err != nil {
		return Snapshot{}, err
	}
	return normalizeSnapshot(snapshot)
}

func (s *Store) ApplyPatch(ctx context.Context, params ApplyPatchParams) (Snapshot, error) {
	if s == nil || s.backend == nil {
		return Snapshot{}, errors.New("maintenance store is not initialized")
	}

	before, err := normalizeSnapshot(params.Before)
	if err != nil {
		return Snapshot{}, err
	}
	after, err := normalizeSnapshot(params.After)
	if err != nil {
		return Snapshot{}, err
	}

	if after.Version != before.Version+1 {
		return Snapshot{}, &ValidationError{Message: "after version must be before version + 1"}
	}
	if after.UpdatedAt.IsZero() {
		return Snapshot{}, &ValidationError{Message: "after updated_at is required"}
	}
	if after.CreatedAt.IsZero() {
		after.CreatedAt = before.CreatedAt
	}
	if after.CreatedAt.IsZero() {
		return Snapshot{}, &ValidationError{Message: "after created_at is required"}
	}

	params.Before = before
	params.After = after
	params.ChangedFields = normalizeChangedFields(params.ChangedFields)
	params.ActorUserID = strings.TrimSpace(params.ActorUserID)
	params.RequestID = strings.TrimSpace(params.RequestID)
	params.Source = strings.TrimSpace(strings.ToLower(params.Source))
	if params.Source == "" {
		params.Source = "web"
	}
	if params.ChangedAt.IsZero() {
		params.ChangedAt = time.Now().UTC()
	}

	snapshot, err := s.backend.ApplyPatch(ctx, params)
	if err != nil {
		return Snapshot{}, err
	}
	return normalizeSnapshot(snapshot)
}

func DefaultData() Data {
	return Data{
		Enabled:           false,
		EstimatedDowntime: defaultEstimatedDowntime,
		PublicMessage:     defaultPublicMessage,
		Services:          defaultServiceOverrides(),
	}
}

func DefaultSnapshot(now time.Time) Snapshot {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Snapshot{
		Data:      DefaultData(),
		Version:   1,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
}

func normalizeSnapshot(snapshot Snapshot) (Snapshot, error) {
	data, err := normalizeData(snapshot.Data)
	if err != nil {
		return Snapshot{}, err
	}

	snapshot.Data = data
	if snapshot.Version < 1 {
		snapshot.Version = 1
	}
	snapshot.CreatedAt = snapshot.CreatedAt.UTC()
	snapshot.UpdatedAt = snapshot.UpdatedAt.UTC()
	snapshot.UpdatedByUserID = strings.TrimSpace(snapshot.UpdatedByUserID)

	return snapshot, nil
}

func normalizeData(data Data) (Data, error) {
	data.EstimatedDowntime = strings.TrimSpace(data.EstimatedDowntime)
	if data.EstimatedDowntime == "" {
		data.EstimatedDowntime = defaultEstimatedDowntime
	}
	if len([]rune(data.EstimatedDowntime)) > 120 {
		return Data{}, &ValidationError{Message: "estimated_downtime must be at most 120 characters"}
	}

	data.PublicMessage = strings.TrimSpace(data.PublicMessage)
	if data.PublicMessage == "" {
		data.PublicMessage = defaultPublicMessage
	}
	if len([]rune(data.PublicMessage)) > 1000 {
		return Data{}, &ValidationError{Message: "public_message must be at most 1000 characters"}
	}

	serviceIndex := map[ServiceKey]ServiceOverride{}
	for _, service := range data.Services {
		normalizedService, err := normalizeServiceOverride(service)
		if err != nil {
			return Data{}, err
		}
		serviceIndex[normalizedService.Key] = normalizedService
	}

	defaults := defaultServiceOverrides()
	normalizedServices := make([]ServiceOverride, 0, len(defaults))
	for _, base := range defaults {
		service, exists := serviceIndex[base.Key]
		if !exists {
			normalizedServices = append(normalizedServices, base)
			continue
		}
		if service.Name == "" {
			service.Name = base.Name
		}
		normalizedServices = append(normalizedServices, service)
	}

	data.Services = normalizedServices
	return data, nil
}

func normalizeServiceOverride(service ServiceOverride) (ServiceOverride, error) {
	service.Key = ServiceKey(strings.TrimSpace(strings.ToLower(string(service.Key))))
	if !IsValidServiceKey(service.Key) {
		return ServiceOverride{}, &ValidationError{Message: fmt.Sprintf("unsupported service key: %s", service.Key)}
	}

	service.Name = strings.TrimSpace(service.Name)
	if service.Name == "" {
		service.Name = serviceName(service.Key)
	}

	service.Status = ServiceStatus(strings.TrimSpace(strings.ToLower(string(service.Status))))
	if service.Status == "" {
		service.Status = StatusActive
	}
	if !IsValidServiceStatus(service.Status) {
		return ServiceOverride{}, &ValidationError{Message: fmt.Sprintf("unsupported service status: %s", service.Status)}
	}

	return service, nil
}

func normalizeChangedFields(fields []string) []string {
	if len(fields) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(fields))
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		clean := strings.TrimSpace(field)
		if clean == "" {
			continue
		}
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		normalized = append(normalized, clean)
	}

	sort.Strings(normalized)
	return normalized
}

func defaultServiceOverrides() []ServiceOverride {
	return []ServiceOverride{
		{Key: ServiceAPIGateway, Name: serviceName(ServiceAPIGateway), Status: StatusActive, Enabled: true},
		{Key: ServicePrimaryDatabase, Name: serviceName(ServicePrimaryDatabase), Status: StatusActive, Enabled: true},
		{Key: ServiceWorkerNodes, Name: serviceName(ServiceWorkerNodes), Status: StatusActive, Enabled: true},
	}
}

func serviceName(key ServiceKey) string {
	switch key {
	case ServiceAPIGateway:
		return "API Gateway"
	case ServicePrimaryDatabase:
		return "Primary Database"
	case ServiceWorkerNodes:
		return "Worker Nodes"
	default:
		return string(key)
	}
}

func IsValidServiceKey(key ServiceKey) bool {
	switch key {
	case ServiceAPIGateway, ServicePrimaryDatabase, ServiceWorkerNodes:
		return true
	default:
		return false
	}
}

func IsValidServiceStatus(status ServiceStatus) bool {
	switch status {
	case StatusActive, StatusMaintenance, StatusScaling, StatusRefreshing:
		return true
	default:
		return false
	}
}
