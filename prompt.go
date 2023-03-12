package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tokenizer "github.com/samber/go-gpt-3-encoder"
	gogpt "github.com/sashabaranov/go-gpt3"
	"gopkg.in/yaml.v3"
)

const DefaultInputMarker = "$INPUT"

type Prompt struct {
	Description string `yaml:"description"`
	InputMarker string `yaml:"input_marker"`
	Messages    []struct {
		Role    string `yaml:"role"`
		Content string `yaml:"content"`
	} `yaml:"messages"`
	Temperature float32 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`
}

func (p *Prompt) CreateMessages(input string) []gogpt.ChatCompletionMessage {
	messages := []gogpt.ChatCompletionMessage{}
	for _, message := range p.Messages {
		// replace input marker with input
		content := strings.ReplaceAll(message.Content, p.InputMarker, input)

		messages = append(messages, gogpt.ChatCompletionMessage{
			Role:    message.Role,
			Content: content,
		})
	}
	return messages
}

// CountTokens counts the number of tokens in the prompt
func (p *Prompt) CountTokens() (int, error) {
	count := 0
	encoder, err := tokenizer.NewEncoder()
	if err != nil {
		return 0, err
	}
	for _, message := range p.Messages {
		// Encode string with GPT tokenizer
		encoded, err := encoder.Encode(message.Content)
		if err != nil {
			return 0, err
		}
		count += len(encoded)
	}
	return count, nil
}

// AllowedInputTokens returns the number of tokens allowed for the input
func (p *Prompt) AllowedInputTokens() (int, error) {
	promptTokens, err := p.CountTokens()
	if err != nil {
		return 0, err
	}
	result := 4096 - (promptTokens + p.MaxTokens)
	if result <= 0 {
		return 0, fmt.Errorf("allowed tokens for input is %d, but it should be greater than 0", result)
	}
	return result, nil
}

func splitStringWithTokensLimit(s string, tokensLimit int) ([]string, error) {
	encoder, err := tokenizer.NewEncoder()
	if err != nil {
		return nil, err
	}
	encoded, err := encoder.Encode(s)
	if err != nil {
		return nil, err
	}
	var parts []string
	for {
		if len(encoded) == 0 {
			break
		}
		if len(encoded) <= tokensLimit {
			parts = append(parts, encoder.Decode(encoded))
			break
		}
		parts = append(parts, encoder.Decode(encoded[:tokensLimit]))
		encoded = encoded[tokensLimit:]
	}
	return parts, nil
}

func (p *Prompt) CreateMessagesWithSplit(input string, maxTokensOverride int) ([][]gogpt.ChatCompletionMessage, error) {
	allowedInputTokens, err := p.AllowedInputTokens()
	if err != nil {
		return nil, err
	}
	inputParts, err := splitStringWithTokensLimit(input, allowedInputTokens)
	if err != nil {
		return nil, err
	}
	messages := [][]gogpt.ChatCompletionMessage{}
	for _, inputPart := range inputParts {
		messages = append(messages, p.CreateMessages(inputPart))
	}
	return messages, nil
}

func NewPromptFromFile(filename string) (*Prompt, error) {
	prompt := &Prompt{}
	if err := ReadYamlFromFile(filename, prompt); err != nil {
		return nil, err
	}
	if prompt.InputMarker == "" {
		prompt.InputMarker = DefaultInputMarker
	}
	return prompt, nil
}

func ReadYamlFromFile(filename string, v interface{}) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(v); err != nil {
		return err
	}
	return nil
}

func ReadPromptsInDir(dirname string) (map[string]*Prompt, error) {
	prompts := map[string]*Prompt{}
	files, err := os.ReadDir(dirname)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		// skip non-yaml files
		if !strings.HasSuffix(file.Name(), ".yaml") && !strings.HasSuffix(file.Name(), ".yml") {
			continue
		}
		prompt, err := NewPromptFromFile(dirname + "/" + file.Name())
		if err != nil {
			return nil, err
		}
		// strip extension
		name := strings.TrimSuffix(file.Name(), ".yaml")
		name = strings.TrimSuffix(name, ".yml")
		prompts[name] = prompt
	}
	return prompts, nil
}

func ReadPrompts() (map[string]*Prompt, error) {
	// Get HOME directory
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dirname := filepath.Join(home, ".aichat", "prompts")
	return ReadPromptsInDir(dirname)
}

func ListPrompts() error {
	prompts, err := ReadPrompts()
	if err != nil {
		return err
	}
	for name, prompt := range prompts {
		fmt.Printf("%s\t%s\n", name, prompt.Description)
	}
	return nil
}
