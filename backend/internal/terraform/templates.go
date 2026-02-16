package terraform

import (
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

func LoadTemplate(name string) (*template.Template, error) {
	filename := fmt.Sprintf("templates/%s.tf.tmpl", name)
	content, err := templateFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read template %s: %w", name, err)
	}

	tmpl, err := template.New(name).Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
	}

	return tmpl, nil
}

func ListTemplates() ([]string, error) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("failed to read templates directory: %w", err)
	}

	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if len(name) > 8 && name[len(name)-8:] == ".tf.tmpl" {
				names = append(names, name[:len(name)-8])
			}
		}
	}
	return names, nil
}

func GetTemplateContent(name string) (string, error) {
	filename := fmt.Sprintf("templates/%s.tf.tmpl", name)
	content, err := templateFS.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", name, err)
	}
	return string(content), nil
}
