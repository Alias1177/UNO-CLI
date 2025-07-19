package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Connect подключается к PostgreSQL
func (p *PostgreSQLMonitor) Connect(connectionString string) error {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	p.db = db
	return nil
}

// GetStats получает статистику PostgreSQL
func (p *PostgreSQLMonitor) GetStats() (*DBStats, error) {
	stats := &DBStats{
		LastUpdate: time.Now(),
	}

	// Активные соединения
	var activeConnections int
	err := p.db.QueryRow(`
		SELECT count(*) 
		FROM pg_stat_activity 
		WHERE state = 'active'
	`).Scan(&activeConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to get active connections: %w", err)
	}
	stats.ActiveConnections = activeConnections

	// Максимальное количество соединений
	var maxConnections int
	err = p.db.QueryRow("SHOW max_connections").Scan(&maxConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to get max connections: %w", err)
	}
	stats.MaxConnections = maxConnections

	// Размер базы данных
	var dbSize string
	err = p.db.QueryRow(`
		SELECT pg_size_pretty(pg_database_size(current_database()))
	`).Scan(&dbSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}
	stats.DatabaseSize = dbSize

	// Количество таблиц
	var tableCount int
	err = p.db.QueryRow(`
		SELECT count(*) 
		FROM information_schema.tables 
		WHERE table_schema = 'public'
	`).Scan(&tableCount)
	if err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}
	stats.TableCount = tableCount

	// Получаем информацию о таблицах
	tables, err := p.GetTables()
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	stats.Tables = tables

	// Получаем медленные запросы
	slowQueries, err := p.GetSlowQueries()
	if err != nil {
		return nil, fmt.Errorf("failed to get slow queries: %w", err)
	}
	stats.SlowQueries = slowQueries

	return stats, nil
}

// GetTables получает информацию о таблицах
func (p *PostgreSQLMonitor) GetTables() ([]TableInfo, error) {
	query := `
		SELECT 
			t.table_name,
			COALESCE(pg_size_pretty(pg_total_relation_size(c.oid)), '0 bytes') as size,
			COALESCE(i.index_count, 0) as index_count
		FROM information_schema.tables t
		LEFT JOIN pg_class c ON c.relname = t.table_name
		LEFT JOIN (
			SELECT 
				tablename,
				count(*) as index_count
			FROM pg_indexes 
			WHERE schemaname = 'public'
			GROUP BY tablename
		) i ON i.tablename = t.table_name
		WHERE t.table_schema = 'public'
		ORDER BY t.table_name
	`

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var table TableInfo
		var size string
		var indexCount int
		var name string

		err := rows.Scan(&name, &size, &indexCount)
		if err != nil {
			continue
		}

		table.Name = name
		table.Size = size
		table.Indexes = indexCount
		table.LastUpdate = time.Now()

		tables = append(tables, table)
	}

	return tables, nil
}

// GetSlowQueries получает медленные запросы (если включен pg_stat_statements)
func (p *PostgreSQLMonitor) GetSlowQueries() ([]SlowQuery, error) {
	// Проверяем, включен ли pg_stat_statements
	var exists bool
	err := p.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
		)
	`).Scan(&exists)
	if err != nil || !exists {
		return []SlowQuery{}, nil
	}

	query := `
		SELECT 
			query,
			mean_time,
			calls,
			usename
		FROM pg_stat_statements 
		JOIN pg_user ON pg_stat_statements.userid = pg_user.usesysid
		WHERE mean_time > 1000  -- медленные запросы (>1 сек)
		ORDER BY mean_time DESC 
		LIMIT 10
	`

	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query slow queries: %w", err)
	}
	defer rows.Close()

	var slowQueries []SlowQuery
	for rows.Next() {
		var query string
		var meanTime float64
		var calls int64
		var username string

		err := rows.Scan(&query, &meanTime, &calls, &username)
		if err != nil {
			continue
		}

		// Обрезаем длинный запрос
		if len(query) > 100 {
			query = query[:97] + "..."
		}

		slowQuery := SlowQuery{
			Query:     query,
			Duration:  time.Duration(meanTime * float64(time.Millisecond)),
			Timestamp: time.Now(),
			User:      username,
		}

		slowQueries = append(slowQueries, slowQuery)
	}

	return slowQueries, nil
}

// KillConnection завершает соединение по ID
func (p *PostgreSQLMonitor) KillConnection(connectionID string) error {
	query := fmt.Sprintf("SELECT pg_terminate_backend(%s)", connectionID)
	_, err := p.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to kill connection %s: %w", connectionID, err)
	}
	return nil
}

// Close закрывает соединение с БД
func (p *PostgreSQLMonitor) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}
