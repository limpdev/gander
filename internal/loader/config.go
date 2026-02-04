package loader

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/limpdev/gander/internal/common"
	"github.com/limpdev/gander/internal/models"
	"gopkg.in/yaml.v3"
)

const CONFIG_INCLUDE_RECURSION_DEPTH_LIMIT = 20

const (
	configVarTypeEnv         = "env"
	configVarTypeSecret      = "secret"
	configVarTypeFileFromEnv = "readFileFromEnv"
)

func NewConfigFromYAML(contents []byte) (*models.Config, error) {
	contents, err := ParseConfigVariables(contents)
	if err != nil {
		return nil, err
	}

	config := &models.Config{}
	config.Server.Port = 8080

	err = yaml.Unmarshal(contents, config)
	if err != nil {
		return nil, err
	}

	if err = IsConfigStateValid(config); err != nil {
		return nil, err
	}

	// Initialize widgets
	// We need to iterate over Pages, then HeadWidgets and Column Widgets
	for p := range config.Pages {
		for w := range config.Pages[p].HeadWidgets {
			if err := config.Pages[p].HeadWidgets[w].Initialize(); err != nil {
				return nil, FormatWidgetInitError(err, config.Pages[p].HeadWidgets[w])
			}
		}

		for c := range config.Pages[p].Columns {
			for w := range config.Pages[p].Columns[c].Widgets {
				if err := config.Pages[p].Columns[c].Widgets[w].Initialize(); err != nil {
					return nil, FormatWidgetInitError(err, config.Pages[p].Columns[c].Widgets[w])
				}
			}
		}
	}

	// Initialize theme
	// Access via config.Theme.ThemeProperties (embedded)
	if err := config.Theme.ThemeProperties.Initialize(); err != nil {
		return nil, fmt.Errorf("initializing theme: %w", err)
	}

	// Initialize theme presets
	// Use Items iterator from OrderedYAMLMap
	for _, preset := range config.Theme.Presets.Items() {
		// preset is a *models.ThemeProperties
		if err := preset.Initialize(); err != nil {
			return nil, fmt.Errorf("initializing theme preset: %w", err)
		}
	}

	return config, nil
}

var (
	envVariableNamePattern = regexp.MustCompile(`^[A-Z0-9_]+$`)
	configVariablePattern  = regexp.MustCompile(`(^|.)\$\{(?:([a-zA-Z]+):)?([a-zA-Z0-9_-]+)\}`)
)

func ParseConfigVariables(contents []byte) ([]byte, error) {
	var err error

	replaced := configVariablePattern.ReplaceAllFunc(contents, func(match []byte) []byte {
		if err != nil {
			return nil
		}

		groups := configVariablePattern.FindSubmatch(match)
		if len(groups) != 4 {
			return match
		}

		prefix := string(groups[1])
		if prefix == `\` {
			if len(match) >= 2 {
				return match[1:]
			} else {
				return nil
			}
		}

		typeAsString, variableName := string(groups[2]), string(groups[3])
		variableType := common.Ternary(typeAsString == "", configVarTypeEnv, typeAsString)

		parsedValue, returnOriginal, localErr := ParseConfigVariableOfType(variableType, variableName)
		if localErr != nil {
			err = fmt.Errorf("parsing variable: %v", localErr)
			return nil
		}

		if returnOriginal {
			return match
		}

		return []byte(prefix + parsedValue)
	})

	if err != nil {
		return nil, err
	}

	return replaced, nil
}

func ParseConfigVariableOfType(variableType, variableName string) (string, bool, error) {
	switch variableType {
	case configVarTypeEnv:
		if !envVariableNamePattern.MatchString(variableName) {
			return "", true, nil
		}

		v, found := os.LookupEnv(variableName)
		if !found {
			return "", false, fmt.Errorf("environment variable %s not found", variableName)
		}

		return v, false, nil
	case configVarTypeSecret:
		secretPath := filepath.Join("/run/secrets", variableName)
		secret, err := os.ReadFile(secretPath)
		if err != nil {
			return "", false, fmt.Errorf("reading secret file: %v", err)
		}

		return strings.TrimSpace(string(secret)), false, nil
	case configVarTypeFileFromEnv:
		if !envVariableNamePattern.MatchString(variableName) {
			return "", true, nil
		}

		filePath, found := os.LookupEnv(variableName)
		if !found {
			return "", false, fmt.Errorf("readFileFromEnv: environment variable %s not found", variableName)
		}

		if !filepath.IsAbs(filePath) {
			return "", false, fmt.Errorf("readFileFromEnv: file path %s is not absolute", filePath)
		}

		fileContents, err := os.ReadFile(filePath)
		if err != nil {
			return "", false, fmt.Errorf("readFileFromEnv: reading file from %s: %v", variableName, err)
		}

		return strings.TrimSpace(string(fileContents)), false, nil
	default:
		return "", true, nil
	}
}

func FormatWidgetInitError(err error, w models.Widget) error {
	return fmt.Errorf("%s widget: %v", w.GetType(), err)
}

var configIncludePattern = regexp.MustCompile(`(?m)^([ \t]*)(?:-[ \t]*)?(?:!|\$)include:[ \t]*(.+)$`)

func ParseYAMLIncludes(mainFilePath string) ([]byte, map[string]struct{}, error) {
	return RecursiveParseYAMLIncludes(mainFilePath, nil, 0)
}

func RecursiveParseYAMLIncludes(mainFilePath string, includes map[string]struct{}, depth int) ([]byte, map[string]struct{}, error) {
	if depth > CONFIG_INCLUDE_RECURSION_DEPTH_LIMIT {
		return nil, nil, fmt.Errorf("recursion depth limit of %d reached", CONFIG_INCLUDE_RECURSION_DEPTH_LIMIT)
	}

	mainFileContents, err := os.ReadFile(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("reading %s: %w", mainFilePath, err)
	}

	mainFileAbsPath, err := filepath.Abs(mainFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("getting absolute path of %s: %w", mainFilePath, err)
	}
	mainFileDir := filepath.Dir(mainFileAbsPath)

	if includes == nil {
		includes = make(map[string]struct{})
	}
	var includesLastErr error

	mainFileContents = configIncludePattern.ReplaceAllFunc(mainFileContents, func(match []byte) []byte {
		if includesLastErr != nil {
			return nil
		}

		matches := configIncludePattern.FindSubmatch(match)
		if len(matches) != 3 {
			includesLastErr = fmt.Errorf("invalid include match: %v", matches)
			return nil
		}

		indent := string(matches[1])
		includeFilePath := strings.TrimSpace(string(matches[2]))
		if !filepath.IsAbs(includeFilePath) {
			includeFilePath = filepath.Join(mainFileDir, includeFilePath)
		}

		var fileContents []byte
		var err error

		includes[includeFilePath] = struct{}{}

		fileContents, includes, err = RecursiveParseYAMLIncludes(includeFilePath, includes, depth+1)
		if err != nil {
			includesLastErr = err
			return nil
		}

		return []byte(common.PrefixStringLines(indent, string(fileContents)))
	})

	if includesLastErr != nil {
		return nil, nil, includesLastErr
	}

	return mainFileContents, includes, nil
}

func ConfigFilesWatcher(
	mainFilePath string,
	lastContents []byte,
	lastIncludes map[string]struct{},
	onChange func(newContents []byte),
	onErr func(error),
) (func() error, error) {
	mainFileAbsPath, err := filepath.Abs(mainFilePath)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of main file: %w", err)
	}

	// TODO: refactor, flaky
	lastIncludes[mainFileAbsPath] = struct{}{}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	updateWatchedFiles := func(previousWatched map[string]struct{}, newWatched map[string]struct{}) {
		for filePath := range previousWatched {
			if _, ok := newWatched[filePath]; !ok {
				watcher.Remove(filePath)
			}
		}

		for filePath := range newWatched {
			if _, ok := previousWatched[filePath]; !ok {
				if err := watcher.Add(filePath); err != nil {
					log.Printf(
						"Could not add file to watcher, changes to this file will not trigger a reload. path: %s, error: %v",
						filePath, err,
					)
				}
			}
		}
	}

	updateWatchedFiles(nil, lastIncludes)

	// needed for lastContents and lastIncludes because they get updated in multiple goroutines
	mu := sync.Mutex{}

	parseAndCompareBeforeCallback := func() {
		currentContents, currentIncludes, err := ParseYAMLIncludes(mainFilePath)
		if err != nil {
			onErr(fmt.Errorf("parsing main file contents for comparison: %w", err))
			return
		}

		// TODO: refactor, flaky
		currentIncludes[mainFileAbsPath] = struct{}{}

		mu.Lock()
		defer mu.Unlock()

		if !maps.Equal(currentIncludes, lastIncludes) {
			updateWatchedFiles(lastIncludes, currentIncludes)
			lastIncludes = currentIncludes
		}

		if !bytes.Equal(lastContents, currentContents) {
			lastContents = currentContents
			onChange(currentContents)
		}
	}

	const debounceDuration = 500 * time.Millisecond
	var debounceTimer *time.Timer
	debouncedParseAndCompareBeforeCallback := func() {
		if debounceTimer != nil {
			debounceTimer.Stop()
			debounceTimer.Reset(debounceDuration)
		} else {
			debounceTimer = time.AfterFunc(debounceDuration, parseAndCompareBeforeCallback)
		}
	}

	deleteLastInclude := func(filePath string) {
		mu.Lock()
		defer mu.Unlock()
		fileAbsPath, _ := filepath.Abs(filePath)
		delete(lastIncludes, fileAbsPath)
	}

	go func() {
		for {
			select {
			case event, isOpen := <-watcher.Events:
				if !isOpen {
					return
				}
				if event.Has(fsnotify.Write) {
					debouncedParseAndCompareBeforeCallback()
				} else if event.Has(fsnotify.Rename) {
					// on linux the file will no longer be watched after a rename, on windows
					// it will continue to be watched with the new name but we have no access to
					// the new name in this event in order to stop watching it manually and match the
					// behavior in linux, may lead to weird unintended behaviors on windows as we're
					// only handling renames from linux's perspective
					// see https://github.com/fsnotify/fsnotify/issues/255

					// remove the old file from our manually tracked includes, calling
					// debouncedParseAndCompareBeforeCallback will re-add it if it's still
					// required after it triggers
					deleteLastInclude(event.Name)

					// wait for file to maybe get created again
					// see https://github.com/glanceapp/glance/pull/358
					for range 10 {
						if _, err := os.Stat(event.Name); err == nil {
							break
						}
						time.Sleep(200 * time.Millisecond)
					}

					debouncedParseAndCompareBeforeCallback()
				} else if event.Has(fsnotify.Remove) {
					deleteLastInclude(event.Name)
					debouncedParseAndCompareBeforeCallback()
				}
			case err, isOpen := <-watcher.Errors:
				if !isOpen {
					return
				}
				onErr(fmt.Errorf("watcher error: %w", err))
			}
		}
	}()

	onChange(lastContents)

	return func() error {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}

		return watcher.Close()
	}, nil
}

func IsConfigStateValid(config *models.Config) error {
	if len(config.Pages) == 0 {
		return fmt.Errorf("no pages configured")
	}

	if len(config.Auth.Users) > 0 && config.Auth.SecretKey == "" {
		return fmt.Errorf("secret-key must be set when users are configured")
	}

	for username := range config.Auth.Users {
		if username == "" {
			return fmt.Errorf("user has no name")
		}

		if len(username) < 3 {
			return errors.New("usernames must be at least 3 characters")
		}

		user := config.Auth.Users[username]

		if user.Password == "" {
			if user.PasswordHashString == "" {
				return fmt.Errorf("user %s must have a password or a password-hash set", username)
			}
		} else if len(user.Password) < 6 {
			return fmt.Errorf("the password for %s must be at least 6 characters", username)
		}
	}

	if config.Server.AssetsPath != "" {
		if _, err := os.Stat(config.Server.AssetsPath); os.IsNotExist(err) {
			return fmt.Errorf("assets directory does not exist: %s", config.Server.AssetsPath)
		}
	}

	for i := range config.Pages {
		page := &config.Pages[i]

		if page.Title == "" {
			return fmt.Errorf("page %d has no name", i+1)
		}

		if page.Width != "" && (page.Width != "wide" && page.Width != "slim" && page.Width != "default") {
			return fmt.Errorf("page %d: width can only be either wide or slim", i+1)
		}

		if page.DesktopNavigationWidth != "" {
			if page.DesktopNavigationWidth != "wide" && page.DesktopNavigationWidth != "slim" && page.DesktopNavigationWidth != "default" {
				return fmt.Errorf("page %d: desktop-navigation-width can only be either wide or slim", i+1)
			}
		}

		if len(page.Columns) == 0 {
			return fmt.Errorf("page %d has no columns", i+1)
		}

		if page.Width == "slim" {
			if len(page.Columns) > 2 {
				return fmt.Errorf("page %d is slim and cannot have more than 2 columns", i+1)
			}
		} else {
			if len(page.Columns) > 3 {
				return fmt.Errorf("page %d has more than 3 columns", i+1)
			}
		}

		columnSizesCount := make(map[string]int)

		for j := range page.Columns {
			column := &page.Columns[j]

			if column.Size != "small" && column.Size != "full" {
				return fmt.Errorf("column %d of page %d: size can only be either small or full", j+1, i+1)
			}

			columnSizesCount[page.Columns[j].Size]++
		}

		full := columnSizesCount["full"]

		if full > 2 || full == 0 {
			return fmt.Errorf("page %d must have either 1 or 2 full width columns", i+1)
		}
	}

	return nil
}
