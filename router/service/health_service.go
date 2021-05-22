package service

type HealthService interface {
	CreateHealthReport() map[string]interface{}
	CreateConnectorsReport() map[string]interface{}
	GetVersion() string
	GetAvailable() bool
	CreateCategorizedConnectorsReport() map[string]interface{}
}
