package widgets

import (
	"context"
	"errors"
	"html/template"
	"time"

	"github.com/limpdev/gander/internal/common"
	"github.com/limpdev/gander/internal/models"
)

var groupWidgetTemplate = common.MustParseTemplate("group.html", "widget-base.html")

type groupWidget struct {
	widgetBase          `yaml:",inline"`
	containerWidgetBase `yaml:",inline"`
}

func (widget *groupWidget) Initialize() error {
	widget.withError(nil)
	widget.HideHeader = true

	for i := range widget.Widgets {
		widget.Widgets[i].SetHideHeader(true)

		if widget.Widgets[i].GetType() == "group" {
			return errors.New("nested groups are not supported")
		} else if widget.Widgets[i].GetType() == "split-column" {
			return errors.New("split columns inside of groups are not supported")
		}
	}

	if err := widget.containerWidgetBase.InitializeWidgets(); err != nil {
		return err
	}

	return nil
}

func (widget *groupWidget) Update(ctx context.Context) {
	widget.containerWidgetBase.Update(ctx)
}

func (widget *groupWidget) SetProviders(providers *models.WidgetProviders) {
	widget.widgetBase.SetProviders(providers)
	widget.containerWidgetBase.SetProviders(providers)
}

func (widget *groupWidget) RequiresUpdate(now *time.Time) bool {
	return widget.containerWidgetBase.RequiresUpdate(now)
}

func (widget *groupWidget) Render() template.HTML {
	return widget.renderTemplate(widget, groupWidgetTemplate)
}
