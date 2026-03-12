package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func (s Service) Validate(tenant string) error {
	rulesDir := filepath.Join(s.Root, "tenants", tenant, "rules")
	if _, err := os.Stat(rulesDir); err != nil {
		return fmt.Errorf("rules 目录不存在: %s", rulesDir)
	}

	packageDoc, err := readYAMLMap(filepath.Join(rulesDir, "package.yaml"))
	if err != nil {
		return err
	}
	inputDoc, err := readYAMLMap(filepath.Join(rulesDir, "input.yaml"))
	if err != nil {
		return err
	}
	workflowDoc, err := readYAMLMap(filepath.Join(rulesDir, "generation-config.yaml"))
	if err != nil {
		return err
	}

	if err := requireMapKeys(packageDoc, "package.yaml", "required_sections", "templates"); err != nil {
		return err
	}
	if err := requireMapKeys(inputDoc, "input.yaml", "file_discovery", "fields"); err != nil {
		return err
	}
	if err := requireNestedKeys(inputDoc, "input.yaml", "file_discovery", "marker"); err != nil {
		return err
	}
	if err := validateInputFields(inputDoc); err != nil {
		return err
	}
	if err := requireMapKeys(workflowDoc, "generation-config.yaml", "planning", "judge", "translation", "render", "display_labels"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "generation-config.yaml", "planning", "system_prompt", "user_prompt"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "generation-config.yaml", "judge", "system_prompt", "user_prompt", "ignore_messages", "skip_sections"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "generation-config.yaml", "translation", "system_prompt"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "generation-config.yaml", "render", "keywords_item_template", "bullets_item_template", "bullets_separator"); err != nil {
		return err
	}
	if err := requireNestedKeys(workflowDoc, "generation-config.yaml", "display_labels", "title", "bullets", "description", "search_terms", "category", "keywords"); err != nil {
		return err
	}
	requiredSections, err := stringListFromMap(packageDoc, "required_sections", "package.yaml")
	if err != nil {
		return err
	}
	templates, ok := packageDoc["templates"].(map[string]any)
	if !ok {
		return fmt.Errorf("package.yaml templates 非法")
	}
	for _, key := range []string{"en", "cn"} {
		path, _ := templates[key].(string)
		if path == "" {
			return fmt.Errorf("package.yaml templates.%s 非法", key)
		}
		if _, err := os.Stat(filepath.Join(rulesDir, path)); err != nil {
			return fmt.Errorf("模板文件不存在: %s", filepath.Join(rulesDir, path))
		}
	}

	sectionDir := filepath.Join(rulesDir, "sections")
	entries, err := os.ReadDir(sectionDir)
	if err != nil {
		return fmt.Errorf("sections 目录不存在: %s", sectionDir)
	}
	found := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(sectionDir, entry.Name())
		doc, err := readYAMLMap(path)
		if err != nil {
			return err
		}
		if err := requireMapKeys(doc, filepath.Base(path), "section", "language", "instruction", "constraints", "execution", "output"); err != nil {
			return err
		}
		section, _ := doc["section"].(string)
		if strings.TrimSpace(section) == "" {
			return fmt.Errorf("%s instruction 不能为空", path)
		}
		if err := requireNestedKeys(doc, filepath.Base(path), "execution", "retries"); err != nil {
			return err
		}
		found[section] = struct{}{}
	}

	missing := make([]string, 0)
	for _, section := range requiredSections {
		if _, ok := found[section]; !ok {
			missing = append(missing, section)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("缺少 section: %v", missing)
	}
	return nil
}

func requireMapKeys(doc map[string]any, file string, keys ...string) error {
	for _, key := range keys {
		if _, ok := doc[key]; !ok {
			return fmt.Errorf("%s 缺少字段: %s", file, key)
		}
	}
	return nil
}

func requireNestedKeys(doc map[string]any, file, key string, keys ...string) error {
	node, ok := doc[key].(map[string]any)
	if !ok {
		return fmt.Errorf("%s %s 结构非法", file, key)
	}
	for _, child := range keys {
		if _, ok := node[child]; !ok {
			return fmt.Errorf("%s 缺少字段: %s.%s", file, key, child)
		}
	}
	return nil
}

func readYAMLMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, fmt.Errorf("%s 根节点必须是对象", path)
	}
	return doc, nil
}

func stringListFromMap(doc map[string]any, key, file string) ([]string, error) {
	raw, ok := doc[key].([]any)
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("%s %s 非法", file, key)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" {
			out = append(out, value)
		}
	}
	return out, nil
}

func stringList(raw any) []string {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(fmt.Sprint(item))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}


func validateInputFields(inputDoc map[string]any) error {
	rawFields, ok := inputDoc["fields"].([]any)
	if !ok || len(rawFields) == 0 {
		return fmt.Errorf("input.yaml fields 非法")
	}
	fieldKeys := make(map[string]struct{}, len(rawFields))
	for idx, raw := range rawFields {
		field, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("input.fields[%d] 结构非法", idx)
		}
		key := strings.TrimSpace(fmt.Sprint(field["key"]))
		if key == "" {
			return fmt.Errorf("input.fields[%d].key 不能为空", idx)
		}
		if _, exists := fieldKeys[key]; exists {
			return fmt.Errorf("input.fields[%s] 重复定义", key)
		}
		fieldKeys[key] = struct{}{}
		fieldType := strings.TrimSpace(fmt.Sprint(field["type"]))
		if fieldType != "scalar" && fieldType != "list" {
			return fmt.Errorf("input.fields[%s].type 非法: %s", key, fieldType)
		}
		capture := strings.TrimSpace(fmt.Sprint(field["capture"]))
		switch capture {
		case "inline_label":
			if _, ok := field["labels"].([]any); !ok {
				return fmt.Errorf("input.fields[%s].labels 非法", key)
			}
		case "heading_section":
			if _, ok := field["heading_aliases"].([]any); !ok {
				return fmt.Errorf("input.fields[%s].heading_aliases 非法", key)
			}
		default:
			return fmt.Errorf("input.fields[%s].capture 非法: %s", key, capture)
		}
	}
	for _, required := range []string{"brand", "keywords", "category"} {
		if _, ok := fieldKeys[required]; !ok {
			return fmt.Errorf("input.fields 缺少字段: %s", required)
		}
	}
	return nil
}
