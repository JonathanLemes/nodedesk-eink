package render

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// Lang holds every user-visible string and the number/date conventions of one
// language. Add a language by adding an entry to langs.
type Lang struct {
	Decimal string
	Days    [7]string
	Months  [12]string
	// DateFormat receives weekday, day and month names.
	DateFormat func(weekday string, day int, month string) string

	Up, System, Storage, LastHour, Network, Docker       string
	Load, Threads, Containers, Running, Kindle, Charging string
	RootDisk                                             string
	CalmTitle, CalmBody, WarnTitle                       string
	HotCPU, HotGPU, FullDisk, FullRAM, Unhealthy         string
	NoGPU                                                string
}

var langs = map[string]*Lang{
	"en": {
		Decimal: ".",
		Days:    [7]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
		Months:  [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		DateFormat: func(w string, d int, m string) string {
			return fmt.Sprintf("%s, %s %d", w, m, d)
		},
		Up: "up %s", System: "System", Storage: "Storage", LastHour: "Last hour",
		Network: "Network", Docker: "Docker", Load: "Load", Threads: "%d threads",
		Containers: "containers", Running: "running", Kindle: "Kindle", Charging: "charging",
		RootDisk:  "System",
		CalmTitle: "All calm over here!", CalmBody: "No alerts. Everything is running smoothly.",
		WarnTitle: "Hey, take a peek:",
		HotCPU:    "CPU is warm (%s)", HotGPU: "GPU is warm (%s)", FullDisk: "%s is %s full",
		FullRAM: "RAM at %s", Unhealthy: "%s is struggling",
		NoGPU: "no GPU",
	},
	"pt": {
		Decimal: ",",
		Days:    [7]string{"Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"},
		Months:  [12]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"},
		DateFormat: func(w string, d int, m string) string {
			return fmt.Sprintf("%s, %d %s", w, d, m)
		},
		Up: "ligado há %s", System: "Sistema", Storage: "Armazenamento", LastHour: "Última hora",
		Network: "Rede", Docker: "Docker", Load: "Carga", Threads: "%d threads",
		Containers: "containers", Running: "rodando", Kindle: "Kindle", Charging: "carregando",
		RootDisk:  "Sistema",
		CalmTitle: "Tudo tranquilo por aqui!", CalmBody: "Nenhum alerta. Está tudo rodando bem.",
		WarnTitle: "Ei, dá uma olhadinha:",
		HotCPU:    "CPU quentinha (%s)", HotGPU: "GPU quentinha (%s)", FullDisk: "%s com %s",
		FullRAM: "RAM em %s", Unhealthy: "%s com problema",
		NoGPU: "sem GPU",
	},
}

// LangFor returns the language for a code like "pt", "pt-BR" or "en_US",
// falling back to English.
func LangFor(code string) *Lang {
	code = strings.ToLower(code)
	if i := strings.IndexAny(code, "-_"); i > 0 {
		code = code[:i]
	}
	if l, ok := langs[code]; ok {
		return l
	}
	return langs["en"]
}

func (l *Lang) num(v float64, decimals int) string {
	return strings.Replace(fmt.Sprintf("%.*f", decimals, v), ".", l.Decimal, 1)
}

func (l *Lang) pct(v float64) string { return fmt.Sprintf("%.0f%%", v) }

func (l *Lang) temp(c float64) string { return fmt.Sprintf("%.0f °C", c) }

// bytes uses SI units (like macOS and the NodeDesk UI): 1 GB = 10^9 bytes.
func (l *Lang) bytes(b uint64) string {
	v := float64(b)
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	i := 0
	for v >= 1000 && i < len(units)-1 {
		v /= 1000
		i++
	}
	switch {
	case i == 0 || v >= 100:
		return l.num(v, 0) + " " + units[i]
	default:
		return l.num(v, 1) + " " + units[i]
	}
}

func (l *Lang) rate(bps float64) string {
	if bps < 1 {
		return "0 B/s"
	}
	return l.bytes(uint64(math.Round(bps))) + "/s"
}

func (l *Lang) date(t time.Time) string {
	return l.DateFormat(l.Days[t.Weekday()], t.Day(), l.Months[t.Month()-1])
}

func duration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dmin", hours, mins)
	default:
		return fmt.Sprintf("%dmin", mins)
	}
}
