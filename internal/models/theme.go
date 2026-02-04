package models

import (
	"fmt"
	"html/template"

	"github.com/limpdev/gander/internal/common"
	"github.com/limpdev/gander/internal/web"
)

var (
	StyleTemplate         = web.MustParseTemplate("theme-style.gotmpl")
	PresetPreviewTemplate = web.MustParseTemplate("theme-preset-preview.html")
)

type ThemeProperties struct {
	BackgroundColor          *HSLColorField `yaml:"background-color"`
	PrimaryColor             *HSLColorField `yaml:"primary-color"`
	PositiveColor            *HSLColorField `yaml:"positive-color"`
	NegativeColor            *HSLColorField `yaml:"negative-color"`
	Light                    bool           `yaml:"light"`
	ContrastMultiplier       float32        `yaml:"contrast-multiplier"`
	TextSaturationMultiplier float32        `yaml:"text-saturation-multiplier"`

	Key                  string        `yaml:"-"`
	CSS                  template.CSS  `yaml:"-"`
	PreviewHTML          template.HTML `yaml:"-"`
	BackgroundColorAsHex string        `yaml:"-"`
}

func (t *ThemeProperties) Initialize() error {
	css, err := common.ExecuteTemplateToString(StyleTemplate, t)
	if err != nil {
		return fmt.Errorf("compiling theme style: %v", err)
	}
	t.CSS = template.CSS(common.WhitespaceAtBeginningOfLinePattern.ReplaceAllString(css, ""))

	previewHTML, err := common.ExecuteTemplateToString(PresetPreviewTemplate, t)
	if err != nil {
		return fmt.Errorf("compiling theme preview: %v", err)
	}
	t.PreviewHTML = template.HTML(previewHTML)

	if t.BackgroundColor != nil {
		t.BackgroundColorAsHex = t.BackgroundColor.ToHex()
	} else {
		t.BackgroundColorAsHex = "#151519"
	}

	return nil
}

func (t1 *ThemeProperties) SameAs(t2 *ThemeProperties) bool {
	if t1 == nil && t2 == nil {
		return true
	}
	if t1 == nil || t2 == nil {
		return false
	}
	if t1.Light != t2.Light {
		return false
	}
	if t1.ContrastMultiplier != t2.ContrastMultiplier {
		return false
	}
	if t1.TextSaturationMultiplier != t2.TextSaturationMultiplier {
		return false
	}
	if !t1.BackgroundColor.SameAs(t2.BackgroundColor) {
		return false
	}
	if !t1.PrimaryColor.SameAs(t2.PrimaryColor) {
		return false
	}
	if !t1.PositiveColor.SameAs(t2.PositiveColor) {
		return false
	}
	if !t1.NegativeColor.SameAs(t2.NegativeColor) {
		return false
	}
	return true
}
