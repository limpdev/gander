package models

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/limpdev/gander/internal/common"
	"gopkg.in/yaml.v3"
)

var hslColorFieldPattern = regexp.MustCompile(`^(?:hsla?\()?([\d\.]+)(?: |,)+([\d\.]+)%?(?: |,)+([\d\.]+)%?\)?$`)

const (
	hslHueMax        = 360
	hslSaturationMax = 100
	hslLightnessMax  = 100
)

type HSLColorField struct {
	H float64
	S float64
	L float64
}

func (c *HSLColorField) String() string {
	return fmt.Sprintf("hsl(%.1f, %.1f%%, %.1f%%)", c.H, c.S, c.L)
}

func (c *HSLColorField) ToHex() string {
	return common.HslToHex(c.H, c.S, c.L)
}

func (c1 *HSLColorField) SameAs(c2 *HSLColorField) bool {
	if c1 == nil && c2 == nil {
		return true
	}
	if c1 == nil || c2 == nil {
		return false
	}
	return c1.H == c2.H && c1.S == c2.S && c1.L == c2.L
}

func (c *HSLColorField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := hslColorFieldPattern.FindStringSubmatch(value)

	if len(matches) != 4 {
		return fmt.Errorf("invalid HSL color format: %s", value)
	}

	hue, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return err
	}

	if hue > hslHueMax {
		return fmt.Errorf("HSL hue must be between 0 and %d", hslHueMax)
	}

	saturation, err := strconv.ParseFloat(matches[2], 64)
	if err != nil {
		return err
	}

	if saturation > hslSaturationMax {
		return fmt.Errorf("HSL saturation must be between 0 and %d", hslSaturationMax)
	}

	lightness, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return err
	}

	if lightness > hslLightnessMax {
		return fmt.Errorf("HSL lightness must be between 0 and %d", hslLightnessMax)
	}

	c.H = hue
	c.S = saturation
	c.L = lightness

	return nil
}

var durationFieldPattern = regexp.MustCompile(`^(\d+)(s|m|h|d)$`)

type DurationField time.Duration

func (d *DurationField) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	matches := durationFieldPattern.FindStringSubmatch(value)

	if len(matches) != 3 {
		return fmt.Errorf("invalid duration format: %s", value)
	}

	duration, err := strconv.Atoi(matches[1])
	if err != nil {
		return err
	}

	switch matches[2] {
	case "s":
		*d = DurationField(time.Duration(duration) * time.Second)
	case "m":
		*d = DurationField(time.Duration(duration) * time.Minute)
	case "h":
		*d = DurationField(time.Duration(duration) * time.Hour)
	case "d":
		*d = DurationField(time.Duration(duration) * 24 * time.Hour)
	}

	return nil
}

type CustomIconField struct {
	URL        template.URL
	AutoInvert bool
}

func NewCustomIconField(value string) CustomIconField {
	const autoInvertPrefix = "auto-invert "
	field := CustomIconField{}

	if strings.HasPrefix(value, autoInvertPrefix) {
		field.AutoInvert = true
		value = strings.TrimPrefix(value, autoInvertPrefix)
	}

	prefix, icon, found := strings.Cut(value, ":")
	if !found {
		field.URL = template.URL(value)
		return field
	}

	basename, ext, found := strings.Cut(icon, ".")
	if !found {
		ext = "svg"
		basename = icon
	}

	if ext != "svg" && ext != "png" {
		ext = "svg"
	}

	switch prefix {
	case "si":
		field.AutoInvert = true
		field.URL = template.URL("https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/" + basename + ".svg")
	case "di":
		field.URL = template.URL("https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/" + ext + "/" + basename + "." + ext)
	case "mdi":
		field.AutoInvert = true
		field.URL = template.URL("https://cdn.jsdelivr.net/npm/@mdi/svg@latest/svg/" + basename + ".svg")
	case "sh":
		field.URL = template.URL("https://cdn.jsdelivr.net/gh/selfhst/icons/" + ext + "/" + basename + "." + ext)
	default:
		field.URL = template.URL(value)
	}

	return field
}

func (i *CustomIconField) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return err
	}

	*i = NewCustomIconField(value)
	return nil
}

type ProxyOptionsField struct {
	URL           string        `yaml:"url"`
	AllowInsecure bool          `yaml:"allow-insecure"`
	Timeout       DurationField `yaml:"timeout"`
	Client        *http.Client  `yaml:"-"`
}

func (p *ProxyOptionsField) UnmarshalYAML(node *yaml.Node) error {
	type proxyOptionsFieldAlias ProxyOptionsField
	alias := (*proxyOptionsFieldAlias)(p)
	var proxyURL string

	if err := node.Decode(&proxyURL); err != nil {
		if err := node.Decode(alias); err != nil {
			return err
		}
	}

	if proxyURL == "" && p.URL == "" {
		return nil
	}

	if p.URL != "" {
		proxyURL = p.URL
	}

	parsedUrl, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("parsing proxy URL: %v", err)
	}

	timeout := common.DefaultClientTimeout
	if p.Timeout > 0 {
		timeout = time.Duration(p.Timeout)
	}

	p.Client = &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(parsedUrl),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.AllowInsecure},
		},
	}

	return nil
}

type QueryParametersField map[string][]string

func (q *QueryParametersField) UnmarshalYAML(node *yaml.Node) error {
	var decoded map[string]any

	if err := node.Decode(&decoded); err != nil {
		return err
	}

	*q = make(QueryParametersField)

	// TODO: refactor the duplication in the switch cases if any more types get added
	for key, value := range decoded {
		switch v := value.(type) {
		case string:
			(*q)[key] = []string{v}
		case int, int8, int16, int32, int64, float32, float64:
			(*q)[key] = []string{fmt.Sprintf("%v", v)}
		case bool:
			(*q)[key] = []string{fmt.Sprintf("%t", v)}
		case []string:
			(*q)[key] = append((*q)[key], v...)
		case []any:
			for _, item := range v {
				switch item := item.(type) {
				case string:
					(*q)[key] = append((*q)[key], item)
				case int, int8, int16, int32, int64, float32, float64:
					(*q)[key] = append((*q)[key], fmt.Sprintf("%v", item))
				case bool:
					(*q)[key] = append((*q)[key], fmt.Sprintf("%t", item))
				default:
					return fmt.Errorf("invalid query parameter value type: %T", item)
				}
			}
		default:
			return fmt.Errorf("invalid query parameter value type: %T", value)
		}
	}

	return nil
}

func (q *QueryParametersField) ToQueryString() string {
	query := url.Values{}

	for key, values := range *q {
		for _, value := range values {
			query.Add(key, value)
		}
	}

	return query.Encode()
}
