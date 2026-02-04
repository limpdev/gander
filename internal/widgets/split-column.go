package widgets

import (
	"context"
	"html/template"
	"time"

	"github.com/limpdev/gander/internal/common"
	"github.com/limpdev/gander/internal/models"
)

var splitColumnWidgetTemplate = common.MustParseTemplate("split-column.html", "widget-base.html")

type splitColumnWidget struct {
	widgetBase          `yaml:",inline"`
	containerWidgetBase `yaml:",inline"`
	MaxColumns          int `yaml:"max-columns"`
}

func (widget *splitColumnWidget) Initialize() error {
	widget.withError(nil).withTitle("Split Column").SetHideHeader(true)

	if err := widget.containerWidgetBase.InitializeWidgets(); err != nil {
		return err
	}

	if widget.MaxColumns < 2 {
		widget.MaxColumns = 2
	}

	return nil
}

func (widget *splitColumnWidget) Update(ctx context.Context) {
	widget.containerWidgetBase.Update(ctx)
}

func (widget *splitColumnWidget) SetProviders(providers *models.WidgetProviders) {
	widget.widgetBase.SetProviders(providers)
	widget.containerWidgetBase.SetProviders(providers)
}

func (widget *splitColumnWidget) RequiresUpdate(now *time.Time) bool {
	return widget.containerWidgetBase.RequiresUpdate(now)
}

func (widget *splitColumnWidget) Render() template.HTML {
	return widget.renderTemplate(widget, splitColumnWidgetTemplate)
}
