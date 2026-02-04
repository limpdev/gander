package widgets

import (
	"context"
	"sync"
	"time"

	"github.com/limpdev/gander/internal/loader"
	"github.com/limpdev/gander/internal/models"
)

type containerWidgetBase struct {
	Widgets models.Widgets `yaml:"widgets"`
}

func (widget *containerWidgetBase) InitializeWidgets() error {
	for i := range widget.Widgets {
		if err := widget.Widgets[i].Initialize(); err != nil {
			return loader.FormatWidgetInitError(err, widget.Widgets[i])
		}
	}

	return nil
}

func (widget *containerWidgetBase) Update(ctx context.Context) {
	var wg sync.WaitGroup
	now := time.Now()

	for w := range widget.Widgets {
		widget := widget.Widgets[w]

		if !widget.RequiresUpdate(&now) {
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			widget.Update(ctx)
		}()
	}

	wg.Wait()
}

func (widget *containerWidgetBase) SetProviders(providers *models.WidgetProviders) {
	for i := range widget.Widgets {
		widget.Widgets[i].SetProviders(providers)
	}
}

func (widget *containerWidgetBase) RequiresUpdate(now *time.Time) bool {
	for i := range widget.Widgets {
		if widget.Widgets[i].RequiresUpdate(now) {
			return true
		}
	}

	return false
}
