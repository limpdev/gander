package common

import (
	"bytes"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/limpdev/gander/internal/web"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func FileServerWithCache(fs http.FileSystem, cacheDuration time.Duration) http.Handler {
	server := http.FileServer(fs)
	cacheControlValue := fmt.Sprintf("public, max-age=%d", int(cacheDuration.Seconds()))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: fix always setting cache control even if the file doesn't exist
		w.Header().Set("Cache-Control", cacheControlValue)
		server.ServeHTTP(w, r)
	})
}

var BuildVersion = "dev"

const DefaultClientTimeout = 5 * time.Second

var (
	SequentialWhitespacePattern        = regexp.MustCompile(`\s+`)
	WhitespaceAtBeginningOfLinePattern = regexp.MustCompile(`(?m)^\s+`)
)

func PercentChange(current, previous float64) float64 {
	if previous == 0 {
		if current == 0 {
			return 0 // 0% change if both are 0
		}
		return 100 // 100% increase if going from 0 to something
	}

	return (current/previous - 1) * 100
}

func ExtractDomainFromUrl(u string) string {
	if u == "" {
		return ""
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}

	return strings.TrimPrefix(strings.ToLower(parsed.Host), "www.")
}

func SvgPolylineCoordsFromYValues(width float64, height float64, values []float64) string {
	if len(values) < 2 {
		return ""
	}

	verticalPadding := height * 0.02
	height -= verticalPadding * 2
	coordinates := make([]string, len(values))
	distanceBetweenPoints := width / float64(len(values)-1)
	min := slices.Min(values)
	max := slices.Max(values)

	for i := range values {
		coordinates[i] = fmt.Sprintf(
			"%.2f,%.2f",
			float64(i)*distanceBetweenPoints,
			((max-values[i])/(max-min))*height+verticalPadding,
		)
	}

	return strings.Join(coordinates, " ")
}

func MaybeCopySliceWithoutZeroValues[T int | float64](values []T) []T {
	if len(values) == 0 {
		return values
	}

	for i := range values {
		if values[i] != 0 {
			continue
		}

		c := make([]T, 0, len(values)-1)

		for i := range values {
			if values[i] != 0 {
				c = append(c, values[i])
			}
		}

		return c
	}

	return values
}

var urlSchemePattern = regexp.MustCompile(`^[a-z]+:\/\/`)

func StripURLScheme(url string) string {
	return urlSchemePattern.ReplaceAllString(url, "")
}

func IsRunningInsideDockerContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func PrefixStringLines(prefix string, s string) string {
	lines := strings.Split(s, "\n")

	for i, line := range lines {
		lines[i] = prefix + line
	}

	return strings.Join(lines, "\n")
}

func LimitStringLength(s string, max int) (string, bool) {
	asRunes := []rune(s)

	if len(asRunes) > max {
		return string(asRunes[:max]), true
	}

	return s, false
}

func ParseRFC3339Time(t string) time.Time {
	parsed, err := time.Parse(time.RFC3339, t)
	if err != nil {
		return time.Now()
	}

	return parsed
}

func NormalizeVersionFormat(version string) string {
	version = strings.ToLower(strings.TrimSpace(version))

	if len(version) > 0 && version[0] != 'v' {
		return "v" + version
	}

	return version
}

func TitleToSlug(s string) string {
	s = strings.ToLower(s)
	s = SequentialWhitespacePattern.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	return s
}

func ExecuteTemplateToString(t *template.Template, data any) (string, error) {
	var b bytes.Buffer
	err := t.Execute(&b, data)
	if err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return b.String(), nil
}

func StringToBool(s string) bool {
	return s == "true" || s == "yes"
}

func ItemAtIndexOrDefault[T any](items []T, index int, def T) T {
	if index >= len(items) {
		return def
	}

	return items[index]
}

func Ternary[T any](condition bool, a, b T) T {
	if condition {
		return a
	}

	return b
}

func HslToHex(h, s, l float64) string {
	s /= 100.0
	l /= 100.0

	var r, g, b float64

	if s == 0 {
		r, g, b = l, l, l
	} else {
		hueToRgb := func(p, q, t float64) float64 {
			if t < 0 {
				t += 1
			}
			if t > 1 {
				t -= 1
			}
			if t < 1.0/6.0 {
				return p + (q-p)*6.0*t
			}
			if t < 1.0/2.0 {
				return q
			}
			if t < 2.0/3.0 {
				return p + (q-p)*(2.0/3.0-t)*6.0
			}
			return p
		}

		q := 0.0
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}

		p := 2*l - q

		h /= 360.0

		r = hueToRgb(p, q, h+1.0/3.0)
		g = hueToRgb(p, q, h)
		b = hueToRgb(p, q, h-1.0/3.0)
	}

	ir := int(math.Round(r * 255.0))
	ig := int(math.Round(g * 255.0))
	ib := int(math.Round(b * 255.0))

	ir = int(math.Max(0, math.Min(255, float64(ir))))
	ig = int(math.Max(0, math.Min(255, float64(ig))))
	ib = int(math.Max(0, math.Min(255, float64(ib))))

	return fmt.Sprintf("#%02x%02x%02x", ir, ig, ib)
}

func MustParseTemplate(primary string, dependencies ...string) *template.Template {
	t, err := template.New(primary).
		Funcs(web.GlobalTemplateFunctions).
		ParseFS(web.TemplateFS, append([]string{primary}, dependencies...)...)

	if err != nil {
		panic(err)
	}

	return t
}

func formatApproxNumber(count int) string {
	if count < 1_000 {
		return strconv.Itoa(count)
	}

	if count < 10_000 {
		return strconv.FormatFloat(float64(count)/1_000, 'f', 1, 64) + "k"
	}

	if count < 1_000_000 {
		return strconv.Itoa(count/1_000) + "k"
	}

	return strconv.FormatFloat(float64(count)/1_000_000, 'f', 1, 64) + "m"
}

func DynamicRelativeTimeAttrs(t interface{ Unix() int64 }) template.HTMLAttr {
	return template.HTMLAttr(`data-dynamic-relative-time="` + strconv.FormatInt(t.Unix(), 10) + `"`)
}

var intl = message.NewPrinter(language.English)
var GlobalTemplateFunctions = template.FuncMap{
	"formatApproxNumber": formatApproxNumber,
	"formatNumber":       intl.Sprint,
	"safeCSS": func(str string) template.CSS {
		return template.CSS(str)
	},
	"safeURL": func(str string) template.URL {
		return template.URL(str)
	},
	"safeHTML": func(str string) template.HTML {
		return template.HTML(str)
	},
	"absInt": func(i int) int {
		return int(math.Abs(float64(i)))
	},
	"formatPrice": func(price float64) string {
		return intl.Sprintf("%.2f", price)
	},
	"formatPriceWithPrecision": func(precision int, price float64) string {
		return intl.Sprintf("%."+strconv.Itoa(precision)+"f", price)
	},
	"dynamicRelativeTimeAttrs": DynamicRelativeTimeAttrs,
	"formatServerMegabytes": func(mb uint64) template.HTML {
		var value string
		var label string

		if mb < 1_000 {
			value = strconv.FormatUint(mb, 10)
			label = "MB"
		} else if mb < 1_000_000 {
			if mb < 10_000 {
				value = fmt.Sprintf("%.1f", float64(mb)/1_000)
			} else {
				value = strconv.FormatUint(mb/1_000, 10)
			}

			label = "GB"
		} else {
			value = fmt.Sprintf("%.1f", float64(mb)/1_000_000)
			label = "TB"
		}

		return template.HTML(value + ` <span class="color-base size-h5">` + label + `</span>`)
	},
}
