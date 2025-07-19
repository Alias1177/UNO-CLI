package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Connect подключается к MySQL
func (m *MySQLMonitor) Connect(connectionString string) error {
	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL: %w", err)
	}

	m.db = db
	return nil
}

// GetStats получает статистику MySQL
func (m *MySQLMonitor) GetStats() (*DBStats, error) {
	stats := &DBStats{
		LastUpdate: time.Now(),
	}

	// Активные соединения
	var activeConnections int
	err := m.db.QueryRow("SHOW STATUS LIKE 'Threads_connected'").Scan(&activeConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to get active connections: %w", err)
	}
	stats.ActiveConnections = activeConnections

	// Максимальное количество соединений
	var maxConnections int
	err = m.db.QueryRow("SHOW VARIABLES LIKE 'max_connections'").Scan(&maxConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to get max connections: %w", err)
	}
	stats.MaxConnections = maxConnections

	// Размер базы данных
	var dbSize string
	err = m.db.QueryRow(`
		SELECT 
			ROUND(SUM(data_length + index_length) / 1024 / 1024, 2) AS 'DB Size in MB'
		FROM information_schema.tables 
		WHERE table_schema = DATABASE()
	`).Scan(&dbSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}
	stats.DatabaseSize = dbSize + " MB"

	// Количество таблиц
	var tableCount int
	err = m.db.QueryRow(`
		SELECT COUNT(*) 
		FROM information_schema.tables 
		WHERE table_schema = DATABASE()
	`).Scan(&tableCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}
	stats.TableCount = tableCount

	// Получаем информацию о таблицах
	tables, err := m.GetTables()
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	stats.Tables = tables

	// Получаем медленные запросы
	slowQueries, err := m.GetSlowQueries()
	if err != nil {
		return nil, fmt.Errorf("failed to get slow queries: %w", err)
	}
	stats.SlowQueries = slowQueries

	return stats, nil
}

// GetTables получает информацию о таблицах
func (m *MySQLMonitor) GetTables() ([]TableInfo, error) {
	query := `
		SELECT 
			table_name,
			table_rows,
			ROUND(((data_length + index_length) / 1024 / 1024), 2) AS 'Size (MB)',
			(SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = t.table_name) as index_count
		FROM information_schema.tables t
		WHERE table_schema = DATABASE()
		ORDER BY table_name
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		var sizeMB float64
		var indexCount int
		var name string

		err := rows.Scan(&name, &sizeMB, &indexCount)
		if err != nil {
			continue
		}

		table.Name = name
		table.Size = fmt.Sprintf("%.2f MB", sizeMB)
		table.Indexes = indexCount
		table.LastUpdate = time.Now()

		tables = append(tables, table)
	}

	return tables, nil
}

// GetSlowQueries получает медленные запросы из slow query log
func (m *MySQLMonitor) GetSlowQueries() ([]SlowQuery, error) {
	// Проверяем, включен ли slow query log
	var slowQueryLog string
	err := m.db.QueryRow("SHOW VARIABLES LIKE 'slow_query_log'").Scan(&slowQueryLog)
	if err != nil || slowQueryLog != "ON" {
		return []SlowQuery{}, nil
	}

	// Получаем информацию о performance schema
	query := `
		SELECT 
			digest_text,
			avg_timer_wait/1000000000 as avg_time_sec,
			count_star,
			schema_name
		FROM performance_schema.events_statements_summary_by_digest
		WHERE avg_timer_wait > 1000000000  -- медленные запросы (>1 сек)
		ORDER BY avg_timer_wait DESC
		LIMIT 10
	`

	rows, err := m.db.Query(query)
	if err != nil {
		return []SlowQuery{}, nil // Performance schema может быть недоступен
	}
	defer rows.Close()

	var slowQueries []SlowQuery
	for rows.Next() {
		var queryText string
		var avgTimeSec float64
		var countStar int64
		var schemaName string

		err := rows.Scan(&queryText, &avgTimeSec, &countStar, &schemaName)
		if err != nil {
			continue
		}

		// Обрезаем длинный запрос
		if len(queryText) > 100 {
			queryText = queryText[:97] + "..."
		}

		slowQuery := SlowQuery{
			Query:     queryText,
			Duration:  time.Duration(avgTimeSec * float64(time.Second)),
			Timestamp: time.Now(),
			User:      schemaName,
		}

		slowQueries = append(slowQueries, slowQuery)
	}

	return slowQueries, nil
}

// KillConnection завершает соединение по ID
func (m *MySQLMonitor) KillConnection(connectionID string) error {
	query := fmt.Sprintf("KILL %s", connectionID)
	_, err := m.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to kill connection %s: %w", connectionID, err)
	}
	return nil
}

// Close закрывает соединение с БД
func (m *MySQLMonitor) Close() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
