package database

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DockerDBMonitor мониторинг БД через Docker exec
type DockerDBMonitor struct {
	containerName string
	user          string
	password      string
	database      string
	port          string
}

// NewDockerDBMonitor создает монитор для подключения через Docker
func NewDockerDBMonitor(containerName, user, password, database, port string) *DockerDBMonitor {
	return &DockerDBMonitor{
		containerName: containerName,
		user:          user,
		password:      password,
		database:      database,
		port:          port,
	}
}

// Connect проверяет доступность контейнера
func (d *DockerDBMonitor) Connect(connectionString string) error {
	// Проверяем, что контейнер существует и запущен
	cmd := exec.Command("docker", "inspect", d.containerName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("container %s not found or not running: %w", d.containerName, err)
	}
	return nil
}

// GetStats получает статистику через Docker exec
func (d *DockerDBMonitor) GetStats() (*DBStats, error) {
	stats := &DBStats{
		LastUpdate: time.Now(),
	}

	// Активные соединения
	activeConnections, err := d.execQuery(`
		SELECT count(*) 
		FROM pg_stat_activity 
		WHERE state = 'active'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active connections: %w", err)
	}
	stats.ActiveConnections = activeConnections

	// Максимальное количество соединений
	maxConnectionsStr, err := d.execQueryString("SHOW max_connections")
	if err != nil {
		return nil, fmt.Errorf("failed to get max connections: %w", err)
	}
	maxConnections, err := strconv.Atoi(maxConnectionsStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse max connections: %w", err)
	}
	stats.MaxConnections = maxConnections

	// Размер базы данных
	dbSize, err := d.execQueryString(`
		SELECT pg_size_pretty(pg_database_size(current_database()))
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}
	stats.DatabaseSize = dbSize

	// Количество таблиц
	tableCount, err := d.execQuery(`
		SELECT count(*) 
		FROM information_schema.tables 
		WHERE table_schema = 'public'
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}
	stats.TableCount = tableCount

	// Получаем информацию о таблицах
	tables, err := d.GetTables()
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}
	stats.Tables = tables

	// Получаем медленные запросы
	slowQueries, err := d.GetSlowQueries()
	if err != nil {
		return nil, fmt.Errorf("failed to get slow queries: %w", err)
	}
	stats.SlowQueries = slowQueries

	return stats, nil
}

// GetTables получает информацию о таблицах
func (d *DockerDBMonitor) GetTables() ([]TableInfo, error) {
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
		LIMIT 20;
	`

	output, err := d.execQueryRaw(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var tables []TableInfo

	// Пропускаем заголовок и разделители
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.Contains(line, "----") || strings.Contains(line, "(20 rows)") {
			continue
		}

		// Ищем строки, которые содержат данные (не заголовки)
		if !strings.Contains(line, "table_name") && !strings.Contains(line, "size") &&
			!strings.Contains(line, "index_count") && !strings.Contains(line, "----") && !strings.Contains(line, "rows)") {

			// Разбиваем по | для правильного парсинга PostgreSQL вывода
			columns := strings.Split(line, "|")
			if len(columns) >= 3 {
				tableName := strings.TrimSpace(columns[0])
				sizeStr := strings.TrimSpace(columns[1])
				indexCountStr := strings.TrimSpace(columns[2])

				indexCount, _ := strconv.Atoi(indexCountStr)
				size := sizeStr

				table := TableInfo{
					Name:       tableName,
					Size:       size,
					Indexes:    indexCount,
					LastUpdate: time.Now(),
				}

				tables = append(tables, table)
			}
		}
	}

	return tables, nil
}

// GetSlowQueries получает медленные запросы
func (d *DockerDBMonitor) GetSlowQueries() ([]SlowQuery, error) {
	// Проверяем, включен ли pg_stat_statements
	exists, err := d.execQueryString(`
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements'
		)
	`)
	if err != nil || exists != "t" {
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
		WHERE mean_time > 1000
		ORDER BY mean_time DESC 
		LIMIT 10
	`

	output, err := d.execQueryRaw(query)
	if err != nil {
		return []SlowQuery{}, nil
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	var slowQueries []SlowQuery

	// Пропускаем заголовок
	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}

		queryText := parts[0]
		meanTime, _ := strconv.ParseFloat(parts[1], 64)
		username := parts[3]

		// Обрезаем длинный запрос
		if len(queryText) > 100 {
			queryText = queryText[:97] + "..."
		}

		slowQuery := SlowQuery{
			Query:     queryText,
			Duration:  time.Duration(meanTime * float64(time.Millisecond)),
			Timestamp: time.Now(),
			User:      username,
		}

		slowQueries = append(slowQueries, slowQuery)
	}

	return slowQueries, nil
}

// KillConnection завершает соединение
func (d *DockerDBMonitor) KillConnection(connectionID string) error {
	query := fmt.Sprintf("SELECT pg_terminate_backend(%s)", connectionID)
	_, err := d.execQueryRaw(query)
	if err != nil {
		return fmt.Errorf("failed to kill connection %s: %w", connectionID, err)
	}
	return nil
}

// Close закрывает соединение (не нужно для Docker exec)
func (d *DockerDBMonitor) Close() error {
	return nil
}

// execQuery выполняет запрос и возвращает int
func (d *DockerDBMonitor) execQuery(query string) (int, error) {
	output, err := d.execQueryRaw(query)
	if err != nil {
		return 0, err
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")

	// Ищем строку с результатом (после заголовка)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "----") || strings.Contains(line, "count") {
			continue
		}
		// Если это число, возвращаем его
		if value, err := strconv.Atoi(line); err == nil {
			return value, nil
		}
	}

	return 0, fmt.Errorf("no numeric result found in output")
}

// execQueryString выполняет запрос и возвращает string
func (d *DockerDBMonitor) execQueryString(query string) (string, error) {
	output, err := d.execQueryRaw(query)
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(output)
	lines := strings.Split(result, "\n")

	// Ищем строку с результатом (после заголовка)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "----") || strings.Contains(line, "pg_size_pretty") ||
			strings.Contains(line, "max_connections") || strings.Contains(line, "pg_database_size") {
			continue
		}
		// Если это не пустая строка и не заголовок, возвращаем её
		if line != "" {
			return line, nil
		}
	}

	return "", fmt.Errorf("no result found in output")
}

// execQueryRaw выполняет запрос через Docker exec
func (d *DockerDBMonitor) execQueryRaw(query string) (string, error) {
	// Создаем временный файл с запросом
	queryFile := fmt.Sprintf("/tmp/query_%d.sql", time.Now().Unix())

	// Создаем файл в контейнере
	createFileCmd := exec.Command("docker", "exec", d.containerName, "bash", "-c",
		fmt.Sprintf("echo \"%s\" > %s", query, queryFile))

	if err := createFileCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create query file: %w", err)
	}

	// Выполняем запрос из файла с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "exec", d.containerName, "bash", "-c",
		fmt.Sprintf("PGPASSWORD=%s psql -U %s -d %s -f %s --no-psqlrc",
			d.password, d.user, d.database, queryFile))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker exec failed: %w, output: %s", err, string(output))
	}

	// Удаляем временный файл
	cleanupCmd := exec.Command("docker", "exec", d.containerName, "rm", queryFile)
	cleanupCmd.Run() // игнорируем ошибку очистки

	result := string(output)
	return result, nil
}
