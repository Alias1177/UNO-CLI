package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/term"
)

type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"msg"`
	JobID   string `json:"jobID,omitempty"`
	Error   string `json:"error,omitempty"`
	Raw     string // оригинальная строка для непарсящихся логов
}

type model struct {
	logEntries     []LogEntry
	ready          bool
	err            error
	containerID    string
	width          int
	height         int
	wrapLines      bool
	showErrorsOnly bool
	logCount       int
	requestedTail  int
	scrollOffset   int // Смещение для прокрутки
}

func initialModel(containerID string) model {
	// Получаем размер терминала
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		width, height = 80, 24 // fallback значения
	}

	return model{
		logEntries:     []LogEntry{},
		ready:          false,
		containerID:    containerID,
		width:          width,
		height:         height,
		wrapLines:      true,
		showErrorsOnly: false,
		logCount:       0,
		requestedTail:  0,
		scrollOffset:   0,
	}
}

func RunLogs(containerID string, errorsOnly bool, tail int) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()

	model := initialModel(containerID)
	model.showErrorsOnly = errorsOnly
	model.requestedTail = tail
	p := tea.NewProgram(model)
	go streamLogs(ctx, p, containerID, tail)

	if err := p.Start(); err != nil {
		return fmt.Errorf("error running logs TUI: %w", err)
	}
	return nil
}

func parseLogLine(line string) LogEntry {
	// Удаляем Docker префикс более аккуратно
	if len(line) >= 8 {
		firstByte := line[0]
		if firstByte == 0x01 || firstByte == 0x02 {
			line = line[8:]
		}
	}

	// Убираем лишние пробелы
	line = strings.TrimSpace(line)

	// Сначала пытаемся найти JSON часть лога
	jsonStart := strings.Index(line, "{")
	if jsonStart != -1 {
		dockerTimestamp := ""
		if jsonStart > 0 {
			dockerTimestamp = strings.TrimSpace(line[:jsonStart])
		}

		jsonPart := line[jsonStart:]

		var logEntry LogEntry
		if err := json.Unmarshal([]byte(jsonPart), &logEntry); err == nil {
			if logEntry.Time == "" && dockerTimestamp != "" {
				logEntry.Time = dockerTimestamp
			}
			return logEntry
		}
	}

	// Если не JSON, используем универсальный парсер
	return parseUniversalLog(line)
}

// Универсальный парсер для различных форматов логов
func parseUniversalLog(line string) LogEntry {
	line = strings.TrimSpace(line)

	// Паттерны для различных форматов логов
	patterns := []struct {
		regex *regexp.Regexp
		parse func([]string) LogEntry
	}{
		// PostgreSQL с Docker timestamp: 2025-07-19T00:26:54.972365013Z 2025-07-19 00:26:54.972 GMT [100] ERROR: relation "mat_object" does not exist at character 15
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) GMT \[(\d+)\] (\w+):\s*(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   matches[4],
					Message: matches[5],
					JobID:   matches[3],
				}
			},
		},
		// PostgreSQL STATEMENT: 2025-07-19T00:26:54.972381388Z 2025-07-19 00:26:54.972 GMT [100] STATEMENT: SELECT * FROM mat_object
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) GMT \[(\d+)\] (STATEMENT):\s*(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   "INFO", // STATEMENT как INFO
					Message: fmt.Sprintf("SQL: %s", matches[5]),
					JobID:   matches[3],
				}
			},
		},
		// PostgreSQL без Docker timestamp: 2025-07-19 00:26:48.173 GMT [1] LOG: message
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}) GMT \[(\d+)\] (\w+):\s*(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   matches[3],
					Message: matches[4],
					JobID:   matches[2],
				}
			},
		},
		// Docker timestamp + PostgreSQL prefix: 2025-07-19T00:26:48.025548052Z postgresql 00:26:48.02 INFO ==> message
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(\w+)\s+(\d{2}:\d{2}:\d{2}\.\d{2})\s+(\w+)\s+(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   matches[4],
					Message: matches[5],
					JobID:   matches[2],
				}
			},
		},
		// PostgreSQL prefix без Docker: postgresql 00:26:48.02 INFO ==> message
		{
			regex: regexp.MustCompile(`^(\w+)\s+(\d{2}:\d{2}:\d{2}\.\d{2})\s+(\w+)\s+(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[2],
					Level:   matches[3],
					Message: matches[4],
					JobID:   matches[1],
				}
			},
		},
		// Пустая строка с Docker timestamp: 2025-07-19T00:26:48.124332552Z
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   "INFO",
					Message: "",
				}
			},
		},
		// Формат [LEVEL]: Docker timestamp + [LEVEL] message
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)?\s*\[(\w+)\]\s*(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   matches[2],
					Message: matches[3],
				}
			},
		},
		// Время + [LEVEL] message: 15:04:05.000 [INFO] message
		{
			regex: regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}(?:\.\d{3})?)\s+\[(\w+)\]\s+(.+)$`),
			parse: func(matches []string) LogEntry {
				return LogEntry{
					Time:    matches[1],
					Level:   matches[2],
					Message: matches[3],
				}
			},
		},
		// Docker timestamp + любое сообщение: 2025-07-19T00:26:48.025548052Z любой текст
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s+(.+)$`),
			parse: func(matches []string) LogEntry {
				message := matches[2]
				level := "INFO"

				// Определяем уровень по ключевым словам
				messageLower := strings.ToLower(message)
				if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "failed") || strings.Contains(messageLower, "exception") {
					level = "ERROR"
				} else if strings.Contains(messageLower, "warn") {
					level = "WARN"
				} else if strings.Contains(messageLower, "debug") {
					level = "DEBUG"
				}

				return LogEntry{
					Time:    matches[1],
					Level:   level,
					Message: message,
				}
			},
		},
		// Полная дата + время + сообщение: 2025-07-19 10:30:45 message
		{
			regex: regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\s+(.+)$`),
			parse: func(matches []string) LogEntry {
				message := matches[2]
				level := "INFO"

				messageLower := strings.ToLower(message)
				if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "failed") {
					level = "ERROR"
				} else if strings.Contains(messageLower, "warn") {
					level = "WARN"
				} else if strings.Contains(messageLower, "debug") {
					level = "DEBUG"
				}

				return LogEntry{
					Time:    matches[1],
					Level:   level,
					Message: message,
				}
			},
		},
		// Только время + сообщение: 15:04:05 message
		{
			regex: regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}(?:\.\d{2,3})?)\s+(.+)$`),
			parse: func(matches []string) LogEntry {
				message := matches[2]
				level := "INFO"

				messageLower := strings.ToLower(message)
				if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "failed") {
					level = "ERROR"
				} else if strings.Contains(messageLower, "warn") {
					level = "WARN"
				} else if strings.Contains(messageLower, "debug") {
					level = "DEBUG"
				}

				return LogEntry{
					Time:    matches[1],
					Level:   level,
					Message: message,
				}
			},
		},
	}

	// Пробуем каждый паттерн
	for _, pattern := range patterns {
		matches := pattern.regex.FindStringSubmatch(line)
		if matches != nil {
			entry := pattern.parse(matches)
			return entry
		}
	}

	// Если ничего не подошло, пытаемся хотя бы извлечь время из строки
	timePattern := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z|\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)?|\d{2}:\d{2}:\d{2}(?:\.\d+)?)`)
	if timeMatch := timePattern.FindStringSubmatch(line); timeMatch != nil {
		// Удаляем время из сообщения
		message := strings.TrimSpace(strings.Replace(line, timeMatch[1], "", 1))

		level := "INFO"
		messageLower := strings.ToLower(message)
		if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "failed") {
			level = "ERROR"
		} else if strings.Contains(messageLower, "warn") {
			level = "WARN"
		} else if strings.Contains(messageLower, "debug") {
			level = "DEBUG"
		}

		return LogEntry{
			Time:    timeMatch[1],
			Level:   level,
			Message: message,
		}
	}

	// Последняя попытка - возвращаем как есть, но пытаемся определить уровень
	level := "INFO"
	lineLower := strings.ToLower(line)
	if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "failed") || strings.Contains(lineLower, "exception") {
		level = "ERROR"
	} else if strings.Contains(lineLower, "warn") {
		level = "WARN"
	} else if strings.Contains(lineLower, "debug") {
		level = "DEBUG"
	}

	return LogEntry{
		Level:   level,
		Message: line,
	}
}

func streamLogs(ctx context.Context, p *tea.Program, containerID string, tail int) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		p.Send(errorMsg{fmt.Errorf("failed to create Docker client: %w", err)})
		return
	}
	defer cli.Close()

	// Check if container exists
	_, err = cli.ContainerInspect(ctx, containerID)
	if err != nil {
		p.Send(errorMsg{fmt.Errorf("container not found: %w", err)})
		return
	}

	// Get container logs
	var tailStr string
	if tail > 0 {
		tailStr = fmt.Sprintf("%d", tail)
	} else {
		tailStr = "all" // Показываем все логи
	}

	reader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       tailStr,
		Timestamps: true,
	})
	if err != nil {
		p.Send(errorMsg{fmt.Errorf("failed to get container logs: %w", err)})
		return
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	lineCount := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			line := scanner.Text()
			logEntry := parseLogLine(line)
			p.Send(logMsg(logEntry))
			lineCount++
		}
	}

	// Отправляем информацию о количестве прочитанных строк
	fmt.Printf("DEBUG: Read %d lines from Docker API\n", lineCount)
	p.Send(infoMsg{fmt.Sprintf("Read %d lines from Docker API", lineCount)})

	if err := scanner.Err(); err != nil {
		p.Send(errorMsg{fmt.Errorf("error reading logs: %w", err)})
	}
}

type logMsg LogEntry
type errorMsg struct{ err error }
type infoMsg struct{ message string }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = v.Width
		m.height = v.Height
	case tea.KeyMsg:
		switch v.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "w":
			m.wrapLines = !m.wrapLines
		case "e":
			m.showErrorsOnly = !m.showErrorsOnly
		case "up", "k":
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
		case "down", "j":
			m.scrollOffset++
		case "home", "g":
			m.scrollOffset = 0
		case "end", "G":
			// Прокручиваем в конец
			m.scrollOffset = len(m.logEntries) - m.height + 5 // +5 для отступа
		case "pageup":
			if m.scrollOffset > m.height {
				m.scrollOffset -= m.height
			} else {
				m.scrollOffset = 0
			}
		case "pagedown":
			m.scrollOffset += m.height
		}
	case logMsg:
		if !m.ready {
			m.ready = true
		}
		m.logEntries = append(m.logEntries, LogEntry(v))
		m.logCount++
		// Убираем ограничение на количество логов в памяти
		// Теперь храним все логи

		// Автоматически прокручиваем в конец при новых логах
		if len(m.logEntries) > m.height-3 {
			m.scrollOffset = len(m.logEntries) - (m.height - 3)
		}
	case errorMsg:
		m.err = v.err
		m.ready = true
	case infoMsg:
		// Просто игнорируем info сообщения для отладки
	}
	return m, nil
}

// Функция для проверки является ли лог ошибкой
func isErrorLog(entry LogEntry) bool {
	// Проверяем уровень лога
	if strings.ToUpper(entry.Level) == "ERROR" {
		return true
	}

	// Проверяем наличие поля Error
	if entry.Error != "" {
		return true
	}

	// Проверяем сообщение на наличие ключевых слов ошибок
	message := strings.ToLower(entry.Message)
	errorKeywords := []string{
		"error", "exception", "failed", "failure", "panic", "fatal",
		"ошибка", "исключение", "сбой", "критическая ошибка",
		"stack trace", "traceback", "crash", "segmentation fault",
		"timeout", "connection refused", "permission denied",
		"not found", "already exists", "invalid", "malformed",
	}

	for _, keyword := range errorKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}

	// Проверяем неструктурированные логи на наличие ошибок
	if entry.Raw != "" {
		rawLower := strings.ToLower(entry.Raw)
		for _, keyword := range errorKeywords {
			if strings.Contains(rawLower, keyword) {
				return true
			}
		}
	}

	return false
}

// Улучшенная функция форматирования времени
func formatTime(timeStr string) string {
	if timeStr == "" {
		return time.Now().Format("15:04:05.000")
	}

	// Парсим время в разных форматах
	formats := []string{
		time.RFC3339Nano,                 // 2025-07-19T00:26:48.173730135Z
		time.RFC3339,                     // 2025-07-19T00:26:48Z
		"2006-01-02T15:04:05.999999999Z", // с наносекундами
		"2006-01-02T15:04:05Z",           // без дробных секунд
		"2006-01-02 15:04:05.999",        // PostgreSQL формат
		"2006-01-02 15:04:05",            // без миллисекунд
		"15:04:05.99",                    // только время с миллисекундами
		"15:04:05",                       // только время
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t.Format("15:04:05.000")
		}
	}

	// Если не удалось распарсить, пробуем извлечь время из строки
	// Ищем паттерн времени в строке
	timePattern := regexp.MustCompile(`(\d{2}:\d{2}:\d{2}(?:\.\d{2,3})?)`)
	if match := timePattern.FindStringSubmatch(timeStr); match != nil {
		timeOnly := match[1]
		// Добираем миллисекунды если их нет
		if !strings.Contains(timeOnly, ".") {
			timeOnly += ".000"
		} else if len(strings.Split(timeOnly, ".")[1]) == 2 {
			timeOnly += "0" // 15:04:05.12 -> 15:04:05.120
		}
		return timeOnly
	}

	return time.Now().Format("15:04:05.000")
}

// Функция для переноса текста по словам
func wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{text}
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		// Если добавление слова превысит ширину
		if currentLine.Len()+len(word)+1 > width {
			if currentLine.Len() > 0 {
				lines = append(lines, currentLine.String())
				currentLine.Reset()
			}

			// Если само слово длиннее ширины, разбиваем его
			if len(word) > width {
				for len(word) > width {
					lines = append(lines, word[:width])
					word = word[width:]
				}
				if len(word) > 0 {
					currentLine.WriteString(word)
				}
			} else {
				currentLine.WriteString(word)
			}
		} else {
			if currentLine.Len() > 0 {
				currentLine.WriteString(" ")
			}
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

func (m model) View() string {
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		return errorStyle.Render(fmt.Sprintf("Error: %v\n\nPress 'q' or Ctrl+C to quit", m.err))
	}

	if !m.ready {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFF00")).
			Bold(true)
		return loadingStyle.Render("Loading logs...\n\nPress 'q' or Ctrl+C to quit")
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Bold(true)

	// Стили для разных уровней логов
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
	debugStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	timeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00AAFF"))

	header := headerStyle.Render(fmt.Sprintf("Container logs: %s | Requested: %d | Loaded: %d logs", m.containerID[:12], m.requestedTail, m.logCount))

	var logLines []string
	var visibleEntries []LogEntry

	// Фильтруем записи
	for _, entry := range m.logEntries {
		// Применяем фильтр ошибок если включен
		if m.showErrorsOnly && !isErrorLog(entry) {
			continue
		}
		visibleEntries = append(visibleEntries, entry)
	}

	// Применяем прокрутку
	start := m.scrollOffset
	availableHeight := m.height - 3 // Оставляем место для header и footer
	end := start + availableHeight
	if end > len(visibleEntries) {
		end = len(visibleEntries)
	}
	if start >= len(visibleEntries) {
		start = len(visibleEntries) - 1
		if start < 0 {
			start = 0
		}
	}

	for _, entry := range visibleEntries[start:end] {

		// УНИФИЦИРОВАННЫЙ ФОРМАТ: HH:MM:SS.mmm [LEVEL] message
		var line strings.Builder

		// 1. Время (всегда форматируем в HH:MM:SS.mmm)
		formattedTime := formatTime(entry.Time)
		line.WriteString(timeStyle.Render(formattedTime))
		line.WriteString(" ")

		// 2. Уровень в квадратных скобках с цветом
		level := normalizeLevel(entry.Level)
		levelStr := fmt.Sprintf("[%s]", level)

		switch level {
		case "ERROR":
			line.WriteString(errorStyle.Render(levelStr))
		case "WARN":
			line.WriteString(warnStyle.Render(levelStr))
		case "INFO":
			line.WriteString(infoStyle.Render(levelStr))
		case "DEBUG":
			line.WriteString(debugStyle.Render(levelStr))
		default:
			line.WriteString(levelStr)
		}

		line.WriteString(" ")

		// 3. Сообщение
		message := extractMessage(entry)

		// Подсвечиваем ошибки в сообщении
		if level == "ERROR" && entry.Error != "" {
			// Разбиваем сообщение на части и выделяем ошибки
			message = highlightErrorInMessage(message, errorStyle)
		}

		line.WriteString(message)

		// Обрабатываем перенос строк
		finalLine := line.String()
		if m.wrapLines {
			wrappedLines := wrapText(finalLine, m.width-2)
			logLines = append(logLines, wrappedLines...)
		} else {
			logLines = append(logLines, finalLine)
		}
	}

	// Ограничиваем количество строк для отображения
	if len(logLines) > availableHeight {
		logLines = logLines[:availableHeight]
	}

	logs := strings.Join(logLines, "\n")

	wrapStatus := ""
	if m.wrapLines {
		wrapStatus = " | Word wrap: ON (press 'w' to toggle)"
	} else {
		wrapStatus = " | Word wrap: OFF (press 'w' to toggle)"
	}

	errorFilterStatus := ""
	if m.showErrorsOnly {
		errorFilterStatus = " | Error filter: ON (press 'e' to toggle)"
	} else {
		errorFilterStatus = " | Error filter: OFF (press 'e' to toggle)"
	}

	scrollStatus := ""
	if len(visibleEntries) > availableHeight {
		scrollStatus = fmt.Sprintf(" | Scroll: %d/%d (↑↓ j/k pgup/pgdn home/end)",
			m.scrollOffset+1, len(visibleEntries))
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Render(fmt.Sprintf("\nPress 'q' or Ctrl+C to quit%s%s%s", wrapStatus, errorFilterStatus, scrollStatus))

	return header + "\n" + logs + footer
}

func normalizeLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))

	// Нормализуем различные варианты
	switch level {
	case "WARNING", "WARN":
		return "WARN"
	case "ERR":
		return "ERROR"
	case "INFORMATION":
		return "INFO"
	case "DBG":
		return "DEBUG"
	case "LOG": // PostgreSQL использует LOG вместо INFO
		return "INFO"
	case "STATEMENT", "STMT":
		return "INFO"
	case "NOTICE":
		return "INFO"
	case "FATAL", "PANIC":
		return "ERROR"
	default:
		if level == "" {
			return "INFO"
		}
		return level
	}
}

func extractMessage(entry LogEntry) string {
	var parts []string

	// Основное сообщение
	if entry.Message != "" {
		parts = append(parts, entry.Message)
	}

	// Добавляем JobID если есть
	if entry.JobID != "" {
		parts = append(parts, fmt.Sprintf("jobID=%s", entry.JobID))
	}

	// Добавляем ошибку если есть
	if entry.Error != "" {
		parts = append(parts, fmt.Sprintf("error=%s", entry.Error))
	}

	// Если ничего нет, возвращаем Raw
	if len(parts) == 0 && entry.Raw != "" {
		return entry.Raw
	}

	return strings.Join(parts, " ")
}

// Функция для подсветки ошибок в сообщении
func highlightErrorInMessage(message string, errorStyle lipgloss.Style) string {
	// Ищем части сообщения, которые содержат error=
	errorPattern := regexp.MustCompile(`(error=.+?)(\s|$)`)

	return errorPattern.ReplaceAllStringFunc(message, func(match string) string {
		return errorStyle.Render(match)
	})
}
