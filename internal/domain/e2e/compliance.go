package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type listingComplianceReport struct {
	Passed     bool                         `json:"passed"`
	Violations []listingComplianceViolation `json:"violations"`
}

type listingComplianceViolation struct {
	Language string `json:"language"`
	Section  string `json:"section"`
	Message  string `json:"message"`
}

type packageSpec struct {
	RequiredSections []string          `yaml:"required_sections"`
	Templates        map[string]string `yaml:"templates"`
}

type sectionRuleSpec struct {
	Section     string                `yaml:"section"`
	Constraints sectionConstraintSpec `yaml:"constraints"`
}

type sectionConstraintSpec struct {
	MinChars         int                           `yaml:"min_chars"`
	MaxChars         int                           `yaml:"max_chars"`
	LineCount        int                           `yaml:"line_count"`
	MinCharsPerLine  int                           `yaml:"min_chars_per_line"`
	MaxCharsPerLine  int                           `yaml:"max_chars_per_line"`
	MinParagraphs    int                           `yaml:"min_paragraphs"`
	MaxParagraphs    int                           `yaml:"max_paragraphs"`
	ToleranceChars   int                           `yaml:"tolerance_chars"`
	Lowercase        bool                          `yaml:"lowercase"`
	HeadingMinWords  int                           `yaml:"heading_min_words"`
	HeadingMaxWords  int                           `yaml:"heading_max_words"`
	KeywordEmbedding keywordEmbeddingConstraintSpec `yaml:"keyword_embedding"`
}

type keywordEmbeddingConstraintSpec struct {
	Lowercase bool `yaml:"lowercase"`
}

var boldPhrasePattern = regexp.MustCompile(`\*\*([^*]+)\*\*`)

type templateSectionSpec struct {
	Heading string
	Key     string
}

type tenantComplianceSpec struct {
	RequiredSections map[string]bool
	TemplateSections map[string][]templateSectionSpec
	SectionRules     map[string]sectionRuleSpec
}

func validateListingCompliance(rulesRoot, tenant, enPath, cnPath string) (listingComplianceReport, error) {
	spec, err := loadTenantComplianceSpec(rulesRoot, tenant)
	if err != nil {
		return listingComplianceReport{}, err
	}

	report := listingComplianceReport{Passed: true}

	enMarkdown, err := os.ReadFile(enPath)
	if err != nil {
		return listingComplianceReport{}, fmt.Errorf("读取英文 markdown 失败: %w", err)
	}
	report.Violations = append(report.Violations, validateMarkdownSections("英文", string(enMarkdown), spec.TemplateSections["en"], spec)...)

	cnMarkdown, err := os.ReadFile(cnPath)
	if err != nil {
		return listingComplianceReport{}, fmt.Errorf("读取中文 markdown 失败: %w", err)
	}
	report.Violations = append(report.Violations, validateMarkdownSections("中文", string(cnMarkdown), spec.TemplateSections["cn"], spec)...)

	if len(report.Violations) > 0 {
		report.Passed = false
	}
	return report, nil
}

func validateSingleListingRegression(rulesRoot, tenant, enPath, cnPath string) (listingComplianceReport, error) {
	spec, err := loadTenantComplianceSpec(rulesRoot, tenant)
	if err != nil {
		return listingComplianceReport{}, err
	}

	report := listingComplianceReport{Passed: true}

	enMarkdown, err := os.ReadFile(enPath)
	if err != nil {
		return listingComplianceReport{}, fmt.Errorf("读取英文 markdown 失败: %w", err)
	}
	report.Violations = append(report.Violations, validateSectionPresence("英文", string(enMarkdown), spec.TemplateSections["en"])...)
	report.Violations = append(report.Violations, validateNamedSectionConstraints("英文", string(enMarkdown), spec.TemplateSections["en"], spec, "bullets")...)

	cnMarkdown, err := os.ReadFile(cnPath)
	if err != nil {
		return listingComplianceReport{}, fmt.Errorf("读取中文 markdown 失败: %w", err)
	}
	report.Violations = append(report.Violations, validateSectionPresence("中文", string(cnMarkdown), spec.TemplateSections["cn"])...)

	if len(report.Violations) > 0 {
		report.Passed = false
	}
	return report, nil
}

func loadTenantComplianceSpec(rulesRoot, tenant string) (tenantComplianceSpec, error) {
	rulesDir := filepath.Join(rulesRoot, "tenants", tenant, "rules")
	rawPackage, err := os.ReadFile(filepath.Join(rulesDir, "package.yaml"))
	if err != nil {
		return tenantComplianceSpec{}, fmt.Errorf("读取 package.yaml 失败: %w", err)
	}

	var pkg packageSpec
	if err := yaml.Unmarshal(rawPackage, &pkg); err != nil {
		return tenantComplianceSpec{}, fmt.Errorf("解析 package.yaml 失败: %w", err)
	}

	spec := tenantComplianceSpec{
		RequiredSections: make(map[string]bool),
		TemplateSections: make(map[string][]templateSectionSpec),
		SectionRules:     make(map[string]sectionRuleSpec),
	}
	for _, name := range pkg.RequiredSections {
		spec.RequiredSections[strings.TrimSpace(name)] = true
	}

	for lang, templatePath := range pkg.Templates {
		sections, err := parseTemplateSections(filepath.Join(rulesDir, templatePath))
		if err != nil {
			return tenantComplianceSpec{}, err
		}
		spec.TemplateSections[lang] = sections
	}

	for _, sectionName := range pkg.RequiredSections {
		if strings.TrimSpace(sectionName) == "" || sectionName == "translation" {
			continue
		}
		rawRule, err := os.ReadFile(filepath.Join(rulesDir, "sections", sectionName+".yaml"))
		if err != nil {
			return tenantComplianceSpec{}, fmt.Errorf("读取 section 规则失败: %w", err)
		}
		var rule sectionRuleSpec
		if err := yaml.Unmarshal(rawRule, &rule); err != nil {
			return tenantComplianceSpec{}, fmt.Errorf("解析 section 规则失败: %w", err)
		}
		spec.SectionRules[sectionName] = rule
	}

	return spec, nil
}

func parseTemplateSections(path string) ([]templateSectionSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取模板失败: %w", err)
	}

	lines := strings.Split(string(raw), "\n")
	sections := make([]templateSectionSpec, 0)
	pendingHeading := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			pendingHeading = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		if pendingHeading == "" || trimmed == "" {
			continue
		}
		start := strings.Index(trimmed, "{{")
		end := strings.Index(trimmed, "}}")
		if start < 0 || end <= start+2 {
			continue
		}
		placeholder := strings.TrimSpace(trimmed[start+2 : end])
		sections = append(sections, templateSectionSpec{
			Heading: pendingHeading,
			Key:     canonicalSectionKey(placeholder),
		})
		pendingHeading = ""
	}
	return sections, nil
}

func canonicalSectionKey(placeholder string) string {
	key := strings.TrimSpace(placeholder)
	key = strings.TrimSuffix(key, "_en")
	key = strings.TrimSuffix(key, "_cn")
	return key
}

func validateMarkdownSections(language string, markdown string, templateSections []templateSectionSpec, spec tenantComplianceSpec) []listingComplianceViolation {
	violations := validateSectionPresence(language, markdown, templateSections)
	if language != "英文" {
		return violations
	}
	parsed := parseMarkdownSections(markdown)
	for _, section := range templateSections {
		content, ok := parsed[section.Heading]
		if !ok || strings.TrimSpace(content) == "" {
			continue
		}
		if spec.RequiredSections[section.Key] {
			violations = append(violations, validateSectionConstraints(language, section.Heading, content, spec.SectionRules[section.Key].Constraints)...)
		}
	}
	return violations
}

func validateSectionPresence(language string, markdown string, templateSections []templateSectionSpec) []listingComplianceViolation {
	parsed := parseMarkdownSections(markdown)
	violations := make([]listingComplianceViolation, 0)
	for _, section := range templateSections {
		content, ok := parsed[section.Heading]
		if ok && strings.TrimSpace(content) != "" {
			continue
		}
		violations = append(violations, listingComplianceViolation{
			Language: language,
			Section:  section.Heading,
			Message:  "section 缺失或为空",
		})
	}
	return violations
}

func validateNamedSectionConstraints(language string, markdown string, templateSections []templateSectionSpec, spec tenantComplianceSpec, sectionKey string) []listingComplianceViolation {
	parsed := parseMarkdownSections(markdown)
	for _, section := range templateSections {
		if section.Key != sectionKey {
			continue
		}
		content, ok := parsed[section.Heading]
		if !ok || strings.TrimSpace(content) == "" {
			return nil
		}
		rule, ok := spec.SectionRules[section.Key]
		if !ok {
			return nil
		}
		return validateSectionConstraints(language, section.Heading, content, rule.Constraints)
	}
	return nil
}

func parseMarkdownSections(markdown string) map[string]string {
	lines := strings.Split(markdown, "\n")
	sections := make(map[string]string)
	current := ""
	buffer := make([]string, 0)
	flush := func() {
		if current == "" {
			return
		}
		sections[current] = strings.TrimSpace(strings.Join(buffer, "\n"))
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			buffer = buffer[:0]
			continue
		}
		if current == "" {
			continue
		}
		buffer = append(buffer, line)
	}
	flush()
	return sections
}

func validateSectionConstraints(language string, heading string, content string, constraints sectionConstraintSpec) []listingComplianceViolation {
	violations := make([]listingComplianceViolation, 0)
	trimmed := strings.TrimSpace(content)
	totalChars := runeLen(trimmed)
	minTotal := constraints.MinChars
	maxTotal := applyMaxTolerance(constraints.MaxChars, constraints.ToleranceChars)
	if minTotal > 0 && totalChars < minTotal {
		violations = append(violations, newViolation(language, heading, fmt.Sprintf("总长度过短: %d < %d", totalChars, minTotal)))
	}
	if maxTotal > 0 && totalChars > maxTotal {
		violations = append(violations, newViolation(language, heading, fmt.Sprintf("总长度过长: %d > %d", totalChars, maxTotal)))
	}

	lines := nonEmptyLines(content)
	if constraints.LineCount > 0 && len(lines) != constraints.LineCount {
		violations = append(violations, newViolation(language, heading, fmt.Sprintf("行数不匹配: %d != %d", len(lines), constraints.LineCount)))
	}
	if constraints.MinCharsPerLine > 0 || constraints.MaxCharsPerLine > 0 {
		minPerLine := constraints.MinCharsPerLine
		maxPerLine := applyMaxTolerance(constraints.MaxCharsPerLine, constraints.ToleranceChars)
		for idx, line := range lines {
			length := runeLen(line)
			if minPerLine > 0 && length < minPerLine {
				violations = append(violations, newViolation(language, heading, fmt.Sprintf("第 %d 行长度过短: %d < %d", idx+1, length, minPerLine)))
			}
			if maxPerLine > 0 && length > maxPerLine {
				violations = append(violations, newViolation(language, heading, fmt.Sprintf("第 %d 行长度过长: %d > %d", idx+1, length, maxPerLine)))
			}
			if constraints.HeadingMinWords > 0 || constraints.HeadingMaxWords > 0 {
				title, _, ok := strings.Cut(line, ":")
				if !ok {
					violations = append(violations, newViolation(language, heading, fmt.Sprintf("第 %d 行缺少“小标题: 正文”结构", idx+1)))
				} else {
					wordCount := len(strings.Fields(strings.TrimSpace(title)))
					if constraints.HeadingMinWords > 0 && wordCount < constraints.HeadingMinWords {
						violations = append(violations, newViolation(language, heading, fmt.Sprintf("第 %d 行小标题词数过少: %d < %d", idx+1, wordCount, constraints.HeadingMinWords)))
					}
					if constraints.HeadingMaxWords > 0 && wordCount > constraints.HeadingMaxWords {
						violations = append(violations, newViolation(language, heading, fmt.Sprintf("第 %d 行小标题词数过多: %d > %d", idx+1, wordCount, constraints.HeadingMaxWords)))
					}
				}
			}
			if constraints.KeywordEmbedding.Lowercase {
				for _, phrase := range extractBoldPhrases(line) {
					if phrase != strings.ToLower(phrase) {
						violations = append(violations, newViolation(language, heading, fmt.Sprintf("第 %d 行加粗关键词不是全小写: %s", idx+1, phrase)))
					}
				}
			}
		}
	}

	paragraphs := nonEmptyParagraphs(content)
	if constraints.MinParagraphs > 0 && len(paragraphs) < constraints.MinParagraphs {
		violations = append(violations, newViolation(language, heading, fmt.Sprintf("段落数过少: %d < %d", len(paragraphs), constraints.MinParagraphs)))
	}
	if constraints.MaxParagraphs > 0 && len(paragraphs) > constraints.MaxParagraphs {
		violations = append(violations, newViolation(language, heading, fmt.Sprintf("段落数过多: %d > %d", len(paragraphs), constraints.MaxParagraphs)))
	}

	if constraints.Lowercase && trimmed != strings.ToLower(trimmed) {
		violations = append(violations, newViolation(language, heading, "内容不是全小写"))
	}

	return violations
}

func extractBoldPhrases(content string) []string {
	matches := boldPhrasePattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		phrase := strings.TrimSpace(match[1])
		if phrase == "" {
			continue
		}
		out = append(out, phrase)
	}
	return out
}

func newViolation(language string, heading string, message string) listingComplianceViolation {
	return listingComplianceViolation{
		Language: language,
		Section:  heading,
		Message:  message,
	}
}

func nonEmptyLines(content string) []string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func nonEmptyParagraphs(content string) []string {
	raw := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(raw))
	for _, paragraph := range raw {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func applyMinTolerance(min int, tolerance int) int {
	if min <= 0 {
		return 0
	}
	if min-tolerance < 0 {
		return 0
	}
	return min - tolerance
}

func applyMaxTolerance(max int, tolerance int) int {
	if max <= 0 {
		return 0
	}
	return max + tolerance
}

func runeLen(s string) int {
	return utf8.RuneCountInString(strings.TrimSpace(s))
}
