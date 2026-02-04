package widgets

import (
	"html/template"

	"github.com/limpdev/gander/internal/common"
)

var todoWidgetTemplate = common.MustParseTemplate("todo.html", "widget-base.html")

type todoWidget struct {
	widgetBase `yaml:",inline"`
	cachedHTML template.HTML `yaml:"-"`
	TodoID     string        `yaml:"id"`
}

func (widget *todoWidget) Initialize() error {
	widget.withTitle("To-do").withError(nil)

	widget.cachedHTML = widget.renderTemplate(widget, todoWidgetTemplate)
	return nil
}

func (widget *todoWidget) Render() template.HTML {
	return widget.cachedHTML
}
