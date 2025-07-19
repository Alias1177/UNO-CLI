package command

import (
	"fmt"
	"os"
	"uno/internal/database"
	"uno/internal/httpR"
	"uno/internal/logs"
	"uno/internal/teas"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "uno",
	Short: "System monitoring CLI",
}

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Live RAM monitor (TUI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(teas.Model{})
		_, err := p.Run()
		return err
	},
}

var traceCmd = &cobra.Command{
	Use:   "trace [url]",
	Short: "Трассировка HTTP-запроса к URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]
		httpR.RunHttpTrace(url)
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs [container_id]",
	Short: "Show Docker container logs (TUI)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		containerID := args[0]
		errorsOnly, _ := cmd.Flags().GetBool("err")
		tail, _ := cmd.Flags().GetInt("tail")
		return logs.RunLogs(containerID, errorsOnly, tail)
	},
}

func init() {
	logsCmd.Flags().BoolP("err", "e", false, "Show only error logs")
	logsCmd.Flags().IntP("tail", "t", 0, "Number of log lines to show (0 = all logs)")
}

// Команды для мониторинга БД
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database monitoring commands",
}

var dbMonitorCmd = &cobra.Command{
	Use:   "monitor [connection_string]",
	Short: "Database monitoring (TUI)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		connectionString := args[0]

		dbType := database.DetectDBType(connectionString)
		monitor, err := database.NewDBMonitor(dbType)
		if err != nil {
			return fmt.Errorf("unsupported database type: %w", err)
		}

		if err := monitor.Connect(connectionString); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer monitor.Close()

		model := database.NewDBModel(monitor)
		p := tea.NewProgram(model)

		_, err = p.Run()
		return err
	},
}

var dbTablesCmd = &cobra.Command{
	Use:   "tables [connection_string]",
	Short: "Show database tables",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		connectionString := args[0]

		dbType := database.DetectDBType(connectionString)
		monitor, err := database.NewDBMonitor(dbType)
		if err != nil {
			return fmt.Errorf("unsupported database type: %w", err)
		}

		if err := monitor.Connect(connectionString); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer monitor.Close()

		tables, err := monitor.GetTables()
		if err != nil {
			return fmt.Errorf("failed to get tables: %w", err)
		}

		fmt.Printf("Таблицы в базе данных:\n\n")
		for _, table := range tables {
			fmt.Printf("📋 %s\n", table.Name)
			fmt.Printf("   Размер: %s | Индексы: %d\n\n",
				table.Size, table.Indexes)
		}

		return nil
	},
}

var dbSlowQueriesCmd = &cobra.Command{
	Use:   "slow-queries [connection_string]",
	Short: "Show slow queries",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		connectionString := args[0]

		dbType := database.DetectDBType(connectionString)
		monitor, err := database.NewDBMonitor(dbType)
		if err != nil {
			return fmt.Errorf("unsupported database type: %w", err)
		}

		if err := monitor.Connect(connectionString); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer monitor.Close()

		slowQueries, err := monitor.GetSlowQueries()
		if err != nil {
			return fmt.Errorf("failed to get slow queries: %w", err)
		}

		if len(slowQueries) == 0 {
			fmt.Println("Медленных запросов не найдено")
			return nil
		}

		fmt.Printf("Медленные запросы:\n\n")
		for i, query := range slowQueries {
			fmt.Printf("%d. %s\n", i+1, query.Query)
			fmt.Printf("   Время: %v | Пользователь: %s\n\n",
				query.Duration, query.User)
		}

		return nil
	},
}

var dbKillCmd = &cobra.Command{
	Use:   "kill [connection_string] [connection_id]",
	Short: "Kill database connection",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		connectionString := args[0]
		connectionID := args[1]

		dbType := database.DetectDBType(connectionString)
		monitor, err := database.NewDBMonitor(dbType)
		if err != nil {
			return fmt.Errorf("unsupported database type: %w", err)
		}

		if err := monitor.Connect(connectionString); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
		defer monitor.Close()

		if err := monitor.KillConnection(connectionID); err != nil {
			return fmt.Errorf("failed to kill connection: %w", err)
		}

		fmt.Printf("Соединение %s завершено\n", connectionID)
		return nil
	},
}

// Docker команды для подключения к БД в контейнере
var dbDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Database monitoring via Docker exec",
}

var dbDockerMonitorCmd = &cobra.Command{
	Use:   "monitor [container_name] [user] [password] [database]",
	Short: "Database monitoring via Docker exec (TUI)",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		containerName := args[0]
		user := args[1]
		password := args[2]
		dbName := args[3]

		monitor := database.NewDockerDBMonitorFromArgs(containerName, user, password, dbName, "5432")

		if err := monitor.Connect(""); err != nil {
			return fmt.Errorf("failed to connect to container: %w", err)
		}

		model := database.NewDBModel(monitor)
		p := tea.NewProgram(model)

		_, err := p.Run()
		return err
	},
}

var dbDockerTablesCmd = &cobra.Command{
	Use:   "tables [container_name] [user] [password] [database]",
	Short: "Show database tables via Docker exec",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		containerName := args[0]
		user := args[1]
		password := args[2]
		dbName := args[3]

		monitor := database.NewDockerDBMonitorFromArgs(containerName, user, password, dbName, "5432")

		if err := monitor.Connect(""); err != nil {
			return fmt.Errorf("failed to connect to container: %w", err)
		}

		tables, err := monitor.GetTables()
		if err != nil {
			return fmt.Errorf("failed to get tables: %w", err)
		}

		fmt.Printf("Таблицы в базе данных:\n\n")
		for _, table := range tables {
			fmt.Printf("📋 %s\n", table.Name)
			fmt.Printf("   Размер: %s | Индексы: %d\n\n",
				table.Size, table.Indexes)
		}

		return nil
	},
}

var dbDockerSlowQueriesCmd = &cobra.Command{
	Use:   "slow-queries [container_name] [user] [password] [database]",
	Short: "Show slow queries via Docker exec",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		containerName := args[0]
		user := args[1]
		password := args[2]
		dbName := args[3]

		monitor := database.NewDockerDBMonitorFromArgs(containerName, user, password, dbName, "5432")

		if err := monitor.Connect(""); err != nil {
			return fmt.Errorf("failed to connect to container: %w", err)
		}

		slowQueries, err := monitor.GetSlowQueries()
		if err != nil {
			return fmt.Errorf("failed to get slow queries: %w", err)
		}

		if len(slowQueries) == 0 {
			fmt.Println("Медленных запросов не найдено")
			return nil
		}

		fmt.Printf("Медленные запросы:\n\n")
		for i, query := range slowQueries {
			fmt.Printf("%d. %s\n", i+1, query.Query)
			fmt.Printf("   Время: %v | Пользователь: %s\n\n",
				query.Duration, query.User)
		}

		return nil
	},
}

func Execute() {
	rootCmd.AddCommand(monitorCmd)
	rootCmd.AddCommand(traceCmd)
	rootCmd.AddCommand(logsCmd)

	// Добавляем команды для мониторинга БД
	dbCmd.AddCommand(dbMonitorCmd)
	dbCmd.AddCommand(dbTablesCmd)
	dbCmd.AddCommand(dbSlowQueriesCmd)
	dbCmd.AddCommand(dbKillCmd)

	// Добавляем Docker команды
	dbDockerCmd.AddCommand(dbDockerMonitorCmd)
	dbDockerCmd.AddCommand(dbDockerTablesCmd)
	dbDockerCmd.AddCommand(dbDockerSlowQueriesCmd)
	dbCmd.AddCommand(dbDockerCmd)

	rootCmd.AddCommand(dbCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
