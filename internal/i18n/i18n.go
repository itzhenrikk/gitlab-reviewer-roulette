// Package i18n provides internationalization support for bot responses in multiple languages.
package i18n

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed locales/*.yaml
var localesFS embed.FS

// Translator handles message translation and template rendering.
type Translator struct {
	messages map[string]string
	lang     string
}

// New creates a new Translator for the specified language. Falls back to English if the specified language is not found.
func New(lang string) (*Translator, error) {
	if lang == "" {
		lang = "en"
	}

	messages, err := loadTranslations(lang)
	if err != nil {
		// Fallback to English if requested language fails
		if lang != "en" {
			messages, err = loadTranslations("en")
			if err != nil {
				return nil, fmt.Errorf("failed to load fallback English translations: %w", err)
			}
			lang = "en"
		} else {
			return nil, fmt.Errorf("failed to load translations: %w", err)
		}
	}

	return &Translator{
		messages: messages,
		lang:     lang,
	}, nil
}

// Get retrieves a translated message by key with optional template data. Returns the key itself if translation not found.
func (t *Translator) Get(key string, data ...map[string]interface{}) string {
	message, ok := t.messages[key]
	if !ok {
		return key // Return key as fallback
	}

	// If no template data provided, return message as-is
	if len(data) == 0 {
		return message
	}

	// Apply template if data provided
	tmpl, err := template.New(key).Parse(message)
	if err != nil {
		// If template parsing fails, return raw message
		return message
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data[0]); err != nil {
		// If execution fails, return raw message
		return message
	}

	return buf.String()
}

// GetWithFallback retrieves a translated message with a custom fallback value.
func (t *Translator) GetWithFallback(key, fallback string, data ...map[string]interface{}) string {
	message, ok := t.messages[key]
	if !ok {
		return fallback
	}

	// If no template data provided, return message as-is
	if len(data) == 0 {
		return message
	}

	// Apply template if data provided
	tmpl, err := template.New(key).Parse(message)
	if err != nil {
		return fallback
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data[0]); err != nil {
		return fallback
	}

	return buf.String()
}

// GetPlural retrieves a pluralized translated message based on count. Uses key for singular (count == 1) and key_plural for plural.
func (t *Translator) GetPlural(key string, count int, data ...map[string]interface{}) string {
	pluralKey := key
	if count != 1 {
		pluralKey = key + "_plural"
	}

	// Merge count into data
	templateData := map[string]interface{}{
		"Count": count,
	}
	if len(data) > 0 {
		for k, v := range data[0] {
			templateData[k] = v
		}
	}

	return t.Get(pluralKey, templateData)
}

// Lang returns the current language code.
func (t *Translator) Lang() string {
	return t.lang
}

// loadTranslations loads translation messages from YAML file.
func loadTranslations(lang string) (map[string]string, error) {
	filename := fmt.Sprintf("locales/%s.yaml", lang)

	data, err := localesFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read locale file %s: %w", filename, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML for %s: %w", filename, err)
	}

	// Flatten nested structure into dot-notation keys
	messages := make(map[string]string)
	flattenMap(raw, "", messages)

	return messages, nil
}

// Example: {roulette: {title: "foo"}} -> {"roulette.title": "foo"}.
func flattenMap(m map[string]interface{}, prefix string, result map[string]string) {
	for key, value := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case map[string]interface{}:
			// Recursively flatten nested maps
			flattenMap(v, fullKey, result)
		case string:
			// Store string values
			result[fullKey] = v
		default:
			// Convert other types to string
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}
}

// ActiveReviewsMessage is a helper to format active reviews count with proper pluralization.
func (t *Translator) ActiveReviewsMessage(count int) string {
	if count == 0 {
		return ""
	}
	return t.GetPlural("roulette.active_reviews", count)
}

// FormatActiveReviews formats the active reviews count for display. Returns empty string if count is 0, otherwise returns " (X active review(s))".
func (t *Translator) FormatActiveReviews(count int) string {
	if count == 0 {
		return ""
	}
	msg := t.ActiveReviewsMessage(count)
	return fmt.Sprintf(" (%s)", msg)
}

// FromTeamMessage formats the "from Team-X" message.
func (t *Translator) FromTeamMessage(teamName string) string {
	return t.Get("roulette.from_team", map[string]interface{}{
		"Team": teamName,
	})
}

// TitleWithNewlines returns the title with proper newlines for formatting.
func (t *Translator) TitleWithNewlines() string {
	return t.Get("roulette.title") + "\n\n"
}
