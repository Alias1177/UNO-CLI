package teas

import (
	"fmt"
	"github.com/shirou/gopsutil/v4/process"
	"strings"
	"time"
	"uno/internal/table"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type Model struct {
	total        string
	available    string
	usedPct      string
	disk         string
	totalDisk    string
	openConns    string
	macAddress   string
	bytesSent    string
	bytesRecv    string
	ip           string
	err          string
	nameP        string
	statusP      []string
	usernameP    string
	cpuP         float64
	processCount int
	threadCount  int
}

func (m Model) Init() tea.Cmd {
	return tick()
}

func Process() (string, []string, string, float64) {
	procs, err := process.Processes()
	if err != nil {
		fmt.Println(err)
		return "N/A", []string{"N/A"}, "N/A", 0
	}

	var maxCPU float64
	var topName, topUser string
	var topStatus []string

	for _, p := range procs {
		cpu, err := p.CPUPercent()
		if err != nil {
			continue
		}

		if cpu > maxCPU {
			name, _ := p.Name()
			status, _ := p.Status()
			user, _ := p.Username()

			maxCPU = cpu
			topName = name
			topStatus = status
			topUser = user
		}
	}

	return topName, topStatus, topUser, maxCPU
}
func ProcessSummary() (procCount int, threadCount int, err error) {
	procs, err := process.Processes()
	if err != nil {
		return 0, 0, err
	}

	procCount = len(procs)
	threadCount = 0

	for _, p := range procs {
		threads, err := p.NumThreads()
		if err != nil {
			continue
		}
		threadCount += int(threads)
	}

	return procCount, threadCount, nil
}

// GetSystemIP gets the first non-loopback IPv4 address
func GetSystemIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "N/A"
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		isLoopback := false
		for _, flag := range iface.Flags {
			if flag == "loopback" {
				isLoopback = true
				break
			}
		}
		if isLoopback {
			continue
		}

		for _, addr := range iface.Addrs {
			ip := extractIP(addr.Addr)
			if isIPv4(ip) && !isLoopbackIP(ip) {
				return ip
			}
		}
	}

	return "N/A"
}

func extractIP(addr string) string {
	if idx := strings.Index(addr, "/"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func isIPv4(ip string) bool {
	return strings.Count(ip, ".") == 3
}

func isLoopbackIP(ip string) bool {
	return strings.HasPrefix(ip, "127.") || ip == "localhost"
}

func CalcDisk() (string, string, error) {
	res, err := disk.Usage("/")
	if err != nil {
		return "N/A", "N/A", err
	}

	diskUsage := fmt.Sprintf("%.2f GB", float64(res.Used)/1e9)
	totalDisk := fmt.Sprintf("%.2f GB", float64(res.Total)/1e9)

	return diskUsage, totalDisk, nil
}

func CalcNet() (mac, conns, sent, recv, ip string, err error) {
	ip = GetSystemIP()

	ifaces, err := net.Interfaces()
	if err != nil {
		return "N/A", "N/A", "N/A", "N/A", ip, err
	}

	mac = "N/A"
	for _, iface := range ifaces {
		if len(iface.HardwareAddr) > 0 && mac == "N/A" {
			mac = iface.HardwareAddr
			break
		}
	}

	connections, err := net.Connections("all")
	if err != nil {
		conns = "N/A"
	} else {
		conns = fmt.Sprintf("%d", len(connections))
	}

	stats, err := net.IOCounters(false)
	if err != nil || len(stats) == 0 {
		sent = "N/A"
		recv = "N/A"
	} else {
		sent = fmt.Sprintf("%.2f MB", float64(stats[0].BytesSent)/1e6)
		recv = fmt.Sprintf("%.2f MB", float64(stats[0].BytesRecv)/1e6)
	}

	return mac, conns, sent, recv, ip, nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	case tickMsg:
		// Update memory info
		v, err := mem.VirtualMemory()
		if err != nil {
			m.err = fmt.Sprintf("Memory error: %v", err)
			return m, tick()
		}

		// Update disk info
		diskUsed, diskTotal, err := CalcDisk()
		if err != nil {
			m.err = fmt.Sprintf("Disk error: %v", err)
		}

		// Update network info
		mac, conns, sent, recv, ip, err := CalcNet()
		if err != nil {
			m.err = fmt.Sprintf("Network error: %v", err)
		}
		nameP, statusP, usernameP, cpuP := Process()

		procCount, threadCount, err := ProcessSummary()
		if err != nil {
			m.err = fmt.Sprintf("Process summary error: %v", err)
		}

		// Update model fields
		m.total = fmt.Sprintf("%.2f GB", float64(v.Total)/1e9)
		m.available = fmt.Sprintf("%.2f GB", float64(v.Available)/1e9)
		m.usedPct = fmt.Sprintf("%.2f %%", v.UsedPercent)
		m.disk = diskUsed
		m.totalDisk = diskTotal
		m.macAddress = mac
		m.openConns = conns
		m.bytesSent = sent
		m.bytesRecv = recv
		m.ip = ip
		m.nameP = nameP
		m.usernameP = usernameP
		m.cpuP = cpuP
		m.statusP = statusP
		m.processCount = procCount
		m.threadCount = threadCount

		return m, tick()
	}
	return m, nil
}

func (m Model) View() string {
	data := [][]string{
		{"Total RAM", m.total},
		{"Available RAM", m.available},
		{"Used RAM", m.usedPct},
		{"────────────────────────", ""},
		{"Disk Used", m.disk},
		{"Total Disk", m.totalDisk},
		{"────────────────────────", ""},
		{"IP Address", m.ip},
		{"MAC Address", m.macAddress},
		{"Open Connections", m.openConns},
		{"Bytes Sent", m.bytesSent},
		{"Bytes Received", m.bytesRecv},
		{"────────────────────────", ""},
		{"Top Process", m.nameP},
		{"Process CPU", fmt.Sprintf("%.2f %%", m.cpuP)},
		{"Process Status", strings.Join(m.statusP, ", ")},
		{"Process User", m.usernameP},
		{"────────────────────────", ""},
		{"Total Processes", fmt.Sprintf("%d", m.processCount)},
		{"Total Threads", fmt.Sprintf("%d", m.threadCount)},
	}

	view := table.RenderTable(data)

	if m.err != "" {
		view += "\n⚠️  " + m.err
	}

	view += "\n\nPress Q/Ctrl+C/Esc to quit..."

	return view
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
