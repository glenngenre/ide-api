package challenges

import (
	"skwtr-ide-backend/challenges/harness"
)

type LanguageConfig struct {
	Judge0ID int
	Name     string
	Builder  harness.HarnessBuilder
}

var languageRegistry = map[string]LanguageConfig{
	"python": {
		Judge0ID: 71,
		Name:     "Python",
		Builder:  &harness.PythonBuilder{},
	},
	"javascript": {
		Judge0ID: 63,
		Name:     "JavaScript",
		Builder:  &harness.JavaScriptBuilder{},
	},
	"typescript": {
		Judge0ID: 74,
		Name:     "TypeScript",
		Builder:  &harness.TypeScriptBuilder{},
	},
	"java": {
		Judge0ID: 62,
		Name:     "Java",
		Builder:  &harness.JavaBuilder{},
	},
}

func GetLanguageConfig(lang string) *LanguageConfig {
	if cfg, ok := languageRegistry[lang]; ok {
		return &cfg
	}
	return nil
}

func IsSupported(lang string) bool {
	_, ok := languageRegistry[lang]
	return ok
}
