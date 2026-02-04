package models

import (
	"html/template"
	"sync"
)

type Config struct {
	Server struct {
		Host       string `yaml:"host"`
		Port       uint16 `yaml:"port"`
		Proxied    bool   `yaml:"proxied"`
		AssetsPath string `yaml:"assets-path"`
		BaseURL    string `yaml:"base-url"`
	} `yaml:"server"`

	Auth struct {
		SecretKey string           `yaml:"secret-key"`
		Users     map[string]*User `yaml:"users"`
	} `yaml:"auth"`

	Document struct {
		Head template.HTML `yaml:"head"`
	} `yaml:"document"`

	Theme struct {
		ThemeProperties `yaml:",inline"`
		CustomCSSFile   string `yaml:"custom-css-file"`

		DisablePicker bool                                     `yaml:"disable-picker"`
		Presets       OrderedYAMLMap[string, *ThemeProperties] `yaml:"presets"`
	} `yaml:"theme"`

	Branding struct {
		HideFooter         bool          `yaml:"hide-footer"`
		CustomFooter       template.HTML `yaml:"custom-footer"`
		LogoText           string        `yaml:"logo-text"`
		LogoURL            string        `yaml:"logo-url"`
		FaviconURL         string        `yaml:"favicon-url"`
		FaviconType        string        `yaml:"-"`
		AppName            string        `yaml:"app-name"`
		AppIconURL         string        `yaml:"app-icon-url"`
		AppBackgroundColor string        `yaml:"app-background-color"`
	} `yaml:"branding"`

	Pages []Page `yaml:"pages"`
}

type User struct {
	Password           string `yaml:"password"`
	PasswordHashString string `yaml:"password-hash"`
	PasswordHash       []byte `yaml:"-"`
}

type Page struct {
	Title                  string  `yaml:"name"`
	Slug                   string  `yaml:"slug"`
	Width                  string  `yaml:"width"`
	DesktopNavigationWidth string  `yaml:"desktop-navigation-width"`
	ShowMobileHeader       bool    `yaml:"show-mobile-header"`
	HideDesktopNavigation  bool    `yaml:"hide-desktop-navigation"`
	CenterVertically       bool    `yaml:"center-vertically"`
	HeadWidgets            Widgets `yaml:"head-widgets"`
	Columns                []struct {
		Size    string  `yaml:"size"`
		Widgets Widgets `yaml:"widgets"`
	} `yaml:"columns"`
	PrimaryColumnIndex int8       `yaml:"-"`
	Mu                 sync.Mutex `yaml:"-"`
}
