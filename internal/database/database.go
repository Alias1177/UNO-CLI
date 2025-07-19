package database

import (
	"database/sql"
	"fmt"
	"time"
)

// DBStats содержит статистику базы данных
type DBStats struct {
	ActiveConnections int
	MaxConnections    int
	QueriesPerSecond  float64
	AvgResponseTime   time.Duration
	DatabaseSize      string
	TableCount        int
	Tables            []TableInfo
	SlowQueries       []SlowQuery
	ErrorCount        int
	LastUpdate        time.Time
}

// TableInfo содержит информацию о таблице
type TableInfo struct {
	Name       string
	Size       string
	Indexes    int
	LastUpdate time.Time
}

// SlowQuery содержит информацию о медленном запросе
type SlowQuery struct {
	Query     string
	Duration  time.Duration
	Timestamp time.Time
	User      string
}

// DBMonitor интерфейс для мониторинга разных типов БД
type DBMonitor interface {
	Connect(connectionString string) error
	GetStats() (*DBStats, error)
	GetTables() ([]TableInfo, error)
	GetSlowQueries() ([]SlowQuery, error)
	KillConnection(connectionID string) error
	Close() error
}

// PostgreSQLMonitor реализация для PostgreSQL
type PostgreSQLMonitor struct {
	db *sql.DB
}

// MySQLMonitor реализация для MySQL
type MySQLMonitor struct {
	db *sql.DB
}

// NewPostgreSQLMonitor создает новый монитор для PostgreSQL
func NewPostgreSQLMonitor() *PostgreSQLMonitor {
	return &PostgreSQLMonitor{}
}

// NewMySQLMonitor создает новый монитор для MySQL
func NewMySQLMonitor() *MySQLMonitor {
	return &MySQLMonitor{}
}

// DetectDBType определяет тип БД по connection string
func DetectDBType(connectionString string) string {
	if len(connectionString) > 0 {
		switch connectionString[0] {
		case 'p':
			if len(connectionString) > 8 && connectionString[:8] == "postgres" {
				return "postgresql"
			}
		case 'm':
			if len(connectionString) > 5 && connectionString[:5] == "mysql" {
				return "mysql"
			}
		}
	}
	return "unknown"
}

// NewDBMonitor создает монитор для соответствующего типа БД
func NewDBMonitor(dbType string) (DBMonitor, error) {
	switch dbType {
	case "postgresql":
		return NewPostgreSQLMonitor(), nil
	case "mysql":
		return NewMySQLMonitor(), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// NewDockerDBMonitorFromArgs создает Docker монитор из аргументов
func NewDockerDBMonitorFromArgs(containerName, user, password, database, port string) DBMonitor {
	return NewDockerDBMonitor(containerName, user, password, database, port)
}
