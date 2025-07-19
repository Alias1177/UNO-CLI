package database

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tickMsg time.Time

type DBModel struct {
	monitor      DBMonitor
	stats        *DBStats
	err          error
	ready        bool
	selectedTab  int
	tabs         []string
	connectionID string
}

func NewDBModel(monitor DBMonitor) *DBModel {
	return &DBModel{
		monitor:     monitor,
		tabs:        []string{"Обзор", "Таблицы", "Медленные запросы"},
		selectedTab: 0,
	}
}

func (m *DBModel) Init() tea.Cmd {
	return tick()
}

func (m *DBModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.selectedTab = (m.selectedTab + 1) % len(m.tabs)
		case "shift+tab":
			m.selectedTab = (m.selectedTab - 1 + len(m.tabs)) % len(m.tabs)
		case "k":
			if m.connectionID != "" {
				if err := m.monitor.KillConnection(m.connectionID); err != nil {
					m.err = err
				}
			}
		}
	case tickMsg:
		if !m.ready {
			m.ready = true
		}

		stats, err := m.monitor.GetStats()
		if err != nil {
			m.err = err
		} else {
			m.stats = stats
			m.err = nil
		}

		return m, tick()
	}
	return m, nil
}

func (m *DBModel) View() string {
	if !m.ready {
		return "Подключение к базе данных..."
	}

	if m.err != nil {
		return fmt.Sprintf("Ошибка: %v\nНажмите q для выхода", m.err)
	}

	if m.stats == nil {
		return "Загрузка статистики..."
	}

	var sb strings.Builder

	// Заголовок
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B6B")).
		Bold(true).
		Render("🗄️  Мониторинг базы данных")
	sb.WriteString(title + "\n\n")

	// Табы
	tabs := m.renderTabs()
	sb.WriteString(tabs + "\n\n")

	// Контент в зависимости от выбранной вкладки
	switch m.selectedTab {
	case 0:
		sb.WriteString(m.renderOverview())
	case 1:
		sb.WriteString(m.renderTables())
	case 2:
		sb.WriteString(m.renderSlowQueries())
	}

	// Помощь
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("Tab: переключение вкладок | q: выход | k: завершить соединение")
	sb.WriteString("\n\n" + help)

	return sb.String()
}

func (m *DBModel) renderTabs() string {
	var tabs []string
	for i, tab := range m.tabs {
		if i == m.selectedTab {
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6B6B")).
				Bold(true).
				Render(tab))
		} else {
			tabs = append(tabs, tab)
		}
	}
	return strings.Join(tabs, " | ")
}

func (m *DBModel) renderOverview() string {
	if m.stats == nil {
		return "Нет данных"
	}

	data := [][]string{
		{"Активные соединения", fmt.Sprintf("%d / %d", m.stats.ActiveConnections, m.stats.MaxConnections)},
		{"Размер БД", m.stats.DatabaseSize},
		{"Количество таблиц", fmt.Sprintf("%d", m.stats.TableCount)},
		{"Обновлено", m.stats.LastUpdate.Format("15:04:05")},
	}

	var sb strings.Builder
	for _, row := range data {
		sb.WriteString(fmt.Sprintf("%-20s: %s\n", row[0], row[1]))
	}

	// Прогресс соединений
	connectionPercent := float64(m.stats.ActiveConnections) / float64(m.stats.MaxConnections)
	progressBar := m.renderProgressBar(connectionPercent, 30)
	sb.WriteString(fmt.Sprintf("\nИспользование соединений: %s %.1f%%\n",
		progressBar, connectionPercent*100))

	return sb.String()
}

func (m *DBModel) renderTables() string {
	if m.stats == nil || len(m.stats.Tables) == 0 {
		return "Нет таблиц"
	}

	var sb strings.Builder
	sb.WriteString("Таблицы:\n\n")

	// Заголовок таблицы
	header := fmt.Sprintf("%-20s %-10s %-15s %-10s", "Имя", "Строк", "Размер", "Индексы")
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render(header) + "\n")
	sb.WriteString(strings.Repeat("-", len(header)) + "\n")

	// Данные таблиц
	for _, table := range m.stats.Tables {
		row := fmt.Sprintf("%-20s %-15s %-10d",
			table.Name, table.Size, table.Indexes)
		sb.WriteString(row + "\n")
	}

	return sb.String()
}

func (m *DBModel) renderSlowQueries() string {
	if m.stats == nil || len(m.stats.SlowQueries) == 0 {
		return "Нет медленных запросов"
	}

	var sb strings.Builder
	sb.WriteString("Медленные запросы:\n\n")

	for i, query := range m.stats.SlowQueries {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, query.Query))
		sb.WriteString(fmt.Sprintf("   Время: %v | Пользователь: %s\n\n",
			query.Duration, query.User))
	}

	return sb.String()
}

func (m *DBModel) renderProgressBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

func tick() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
