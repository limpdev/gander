package models

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"math"
	"net/http"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

var widgetIDCounter atomic.Uint64

type Widget interface {
	// These need to be exported because they get called in templates
	Render() template.HTML
	GetType() string
	GetID() uint64

	Initialize() error
	RequiresUpdate(*time.Time) bool
	SetProviders(*WidgetProviders)
	Update(context.Context)
	SetID(uint64)
	HandleRequest(w http.ResponseWriter, r *http.Request)
	SetHideHeader(bool)
}

type Widgets []Widget

// Registry for widget factories
var widgetFactories = make(map[string]func() Widget)

func RegisterWidget(name string, factory func() Widget) {
	widgetFactories[name] = factory
}

func NewWidget(widgetType string) (Widget, error) {
	if widgetType == "" {
		return nil, errors.New("widget 'type' property is empty or not specified")
	}

	factory, ok := widgetFactories[widgetType]
	if !ok {
		return nil, fmt.Errorf("unknown widget type: %s", widgetType)
	}

	w := factory()
	w.SetID(widgetIDCounter.Add(1))

	return w, nil
}

func (w *Widgets) UnmarshalYAML(node *yaml.Node) error {
	var nodes []yaml.Node

	if err := node.Decode(&nodes); err != nil {
		return err
	}

	for _, node := range nodes {
		meta := struct {
			Type string `yaml:"type"`
		}{}

		if err := node.Decode(&meta); err != nil {
			return err
		}

		widget, err := NewWidget(meta.Type)
		if err != nil {
			return fmt.Errorf("line %d: %w", node.Line, err)
		}

		if err = node.Decode(widget); err != nil {
			return err
		}

		*w = append(*w, widget)
	}

	return nil
}

type CacheType int

const (
	CacheTypeInfinite CacheType = iota
	CacheTypeDuration
	CacheTypeOnTheHour
)

type WidgetBase struct {
	ID                  uint64           `yaml:"-"`
	Providers           *WidgetProviders `yaml:"-"`
	Type                string           `yaml:"type"`
	Title               string           `yaml:"title"`
	TitleURL            string           `yaml:"title-url"`
	HideHeader          bool             `yaml:"hide-header"`
	CSSClass            string           `yaml:"css-class"`
	CustomCacheDuration DurationField    `yaml:"cache"`
	ContentAvailable    bool             `yaml:"-"`
	WIP                 bool             `yaml:"-"`
	Error               error            `yaml:"-"`
	Notice              error            `yaml:"-"`
	templateBuffer      bytes.Buffer     `yaml:"-"`
	cacheDuration       time.Duration    `yaml:"-"`
	cacheType           CacheType        `yaml:"-"`
	nextUpdate          time.Time        `yaml:"-"`
	updateRetriedTimes  int              `yaml:"-"`
}

type WidgetProviders struct {
	AssetResolver func(string) string
}

func (w *WidgetBase) RequiresUpdate(now *time.Time) bool {
	if w.cacheType == CacheTypeInfinite {
		return false
	}

	if w.nextUpdate.IsZero() {
		return true
	}

	return now.After(w.nextUpdate)
}

func (w *WidgetBase) IsWIP() bool {
	return w.WIP
}

func (w *WidgetBase) Update(ctx context.Context) {

}

func (w *WidgetBase) GetID() uint64 {
	return w.ID
}

func (w *WidgetBase) SetID(id uint64) {
	w.ID = id
}

func (w *WidgetBase) SetHideHeader(value bool) {
	w.HideHeader = value
}

func (widget *WidgetBase) HandleRequest(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (w *WidgetBase) GetType() string {
	return w.Type
}

func (w *WidgetBase) SetProviders(providers *WidgetProviders) {
	w.Providers = providers
}

func (w *WidgetBase) RenderTemplate(data any, t *template.Template) template.HTML {
	w.templateBuffer.Reset()
	err := t.Execute(&w.templateBuffer, data)
	if err != nil {
		w.ContentAvailable = false
		w.Error = err

		slog.Error("Failed to render template", "error", err)

		// need to immediately re-render with the error,
		// otherwise risk breaking the page since the widget
		// will likely be partially rendered with tags not closed.
		w.templateBuffer.Reset()
		err2 := t.Execute(&w.templateBuffer, data)

		if err2 != nil {
			slog.Error("Failed to render error within widget", "error", err2, "initial_error", err)
			w.templateBuffer.Reset()
		}
	}

	return template.HTML(w.templateBuffer.String())
}

func (w *WidgetBase) WithTitle(title string) *WidgetBase {
	if w.Title == "" {
		w.Title = title
	}

	return w
}

func (w *WidgetBase) WithTitleURL(titleURL string) *WidgetBase {
	if w.TitleURL == "" {
		w.TitleURL = titleURL
	}

	return w
}

func (w *WidgetBase) WithCacheDuration(duration time.Duration) *WidgetBase {
	w.cacheType = CacheTypeDuration

	if duration == -1 || w.CustomCacheDuration == 0 {
		w.cacheDuration = duration
	} else {
		w.cacheDuration = time.Duration(w.CustomCacheDuration)
	}

	return w
}

func (w *WidgetBase) WithCacheOnTheHour() *WidgetBase {
	w.cacheType = CacheTypeOnTheHour

	return w
}

func (w *WidgetBase) WithNotice(err error) *WidgetBase {
	w.Notice = err

	return w
}

func (w *WidgetBase) WithError(err error) *WidgetBase {
	if err == nil && !w.ContentAvailable {
		w.ContentAvailable = true
	}

	w.Error = err

	return w
}

func (w *WidgetBase) CanContinueUpdateAfterHandlingErr(err error) bool {
	// TODO errors
	// if errors.Is(err, errPartialContent) { ... }
	// I need errPartialContent from somewhere.
	// It was in widgets/utils.go. I should move it to models or common.

	if err != nil {
		w.ScheduleEarlyUpdate()
		// Simplification for now, assuming logic handling is adjusted
		w.WithError(err)
		w.WithNotice(nil)
		return false
	}

	w.WithNotice(nil)
	w.WithError(nil)
	w.ScheduleNextUpdate()
	return true
}

func (w *WidgetBase) GetNextUpdateTime() time.Time {
	now := time.Now()

	if w.cacheType == CacheTypeDuration {
		return now.Add(w.cacheDuration)
	}

	if w.cacheType == CacheTypeOnTheHour {
		return now.Add(time.Duration(
			((60-now.Minute())*60)-now.Second(),
		) * time.Second)
	}

	return time.Time{}
}

func (w *WidgetBase) ScheduleNextUpdate() *WidgetBase {
	w.nextUpdate = w.GetNextUpdateTime()
	w.updateRetriedTimes = 0

	return w
}

func (w *WidgetBase) ScheduleEarlyUpdate() *WidgetBase {
	w.updateRetriedTimes++

	if w.updateRetriedTimes > 5 {
		w.updateRetriedTimes = 5
	}

	nextEarlyUpdate := time.Now().Add(time.Duration(math.Pow(float64(w.updateRetriedTimes), 2)) * time.Minute)
	nextUsualUpdate := w.GetNextUpdateTime()

	if nextEarlyUpdate.After(nextUsualUpdate) {
		w.nextUpdate = nextUsualUpdate
	} else {
		w.nextUpdate = nextEarlyUpdate
	}

	return w
}
