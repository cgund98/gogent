package openai

import (
	"encoding/json"
	"fmt"

	openaisdk "github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"

	"github.com/cgund98/gogent"
)

func toChatCompletionNewParams(settings modelSettings, tools []gogent.Tool, history []gogent.Message) (openaisdk.ChatCompletionNewParams, error) {
	messages, err := toSDKChatMessages(settings.systemPrompt, history)
	if err != nil {
		return openaisdk.ChatCompletionNewParams{}, err
	}

	params := openaisdk.ChatCompletionNewParams{
		Model:    openaisdk.ChatModel(settings.model),
		Messages: messages,
	}

	if settings.temperature != nil {
		params.Temperature = openaisdk.Float(*settings.temperature)
	}
	if settings.topP != nil {
		params.TopP = openaisdk.Float(*settings.topP)
	}
	if settings.maxTokens != nil {
		params.MaxTokens = openaisdk.Int(int64(*settings.maxTokens))
	}
	if settings.frequencyPenalty != nil {
		params.FrequencyPenalty = openaisdk.Float(*settings.frequencyPenalty)
	}
	if settings.presencePenalty != nil {
		params.PresencePenalty = openaisdk.Float(*settings.presencePenalty)
	}
	if settings.seed != nil {
		params.Seed = openaisdk.Int(*settings.seed)
	}
	if settings.user != "" {
		params.User = openaisdk.String(settings.user)
	}
	if settings.parallelToolCalls != nil {
		params.ParallelToolCalls = openaisdk.Bool(*settings.parallelToolCalls)
	}
	if len(settings.stopSequences) == 1 {
		params.Stop = openaisdk.ChatCompletionNewParamsStopUnion{
			OfString: openaisdk.String(settings.stopSequences[0]),
		}
	} else if len(settings.stopSequences) > 1 {
		params.Stop = openaisdk.ChatCompletionNewParamsStopUnion{
			OfStringArray: settings.stopSequences,
		}
	}
	if settings.toolChoice != nil {
		params.ToolChoice = openaisdk.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: openaisdk.String(*settings.toolChoice),
		}
	}
	if len(settings.responseFormat) > 0 {
		responseFormat, err := toSDKResponseFormat(settings.responseFormat)
		if err != nil {
			return openaisdk.ChatCompletionNewParams{}, err
		}
		params.ResponseFormat = responseFormat
	}
	if len(tools) > 0 {
		sdkTools, err := toSDKTools(tools)
		if err != nil {
			return openaisdk.ChatCompletionNewParams{}, err
		}
		params.Tools = sdkTools
	}

	return params, nil
}

func toSDKChatMessages(systemPrompt string, history []gogent.Message) ([]openaisdk.ChatCompletionMessageParamUnion, error) {
	wireMessages, err := toOpenAIChatMessages(systemPrompt, history)
	if err != nil {
		return nil, err
	}

	messages := make([]openaisdk.ChatCompletionMessageParamUnion, len(wireMessages))
	for i, message := range wireMessages {
		sdkMessage, err := wireMessageToSDK(message)
		if err != nil {
			return nil, err
		}
		messages[i] = sdkMessage
	}

	return messages, nil
}

func wireMessageToSDK(message openAIChatMessage) (openaisdk.ChatCompletionMessageParamUnion, error) {
	switch message.Role {
	case "system":
		if message.Content == nil {
			return openaisdk.ChatCompletionMessageParamUnion{}, fmt.Errorf("openai: system message is missing content")
		}
		return openaisdk.SystemMessage(*message.Content), nil
	case "user":
		if message.Content == nil {
			return openaisdk.ChatCompletionMessageParamUnion{}, fmt.Errorf("openai: user message is missing content")
		}
		return openaisdk.UserMessage(*message.Content), nil
	case "tool":
		if message.Content == nil {
			return openaisdk.ChatCompletionMessageParamUnion{}, fmt.Errorf("openai: tool message is missing content")
		}
		return openaisdk.ToolMessage(*message.Content, message.ToolCallID), nil
	case "assistant":
		return wireAssistantMessageToSDK(message), nil
	default:
		return openaisdk.ChatCompletionMessageParamUnion{}, fmt.Errorf("openai: unsupported message role %q", message.Role)
	}
}

func wireAssistantMessageToSDK(message openAIChatMessage) openaisdk.ChatCompletionMessageParamUnion {
	if len(message.ToolCalls) == 0 {
		if message.Content == nil || *message.Content == "" {
			return openaisdk.ChatCompletionMessageParamOfAssistant("")
		}
		return openaisdk.ChatCompletionMessageParamOfAssistant(*message.Content)
	}

	assistant := openaisdk.ChatCompletionAssistantMessageParam{}
	if message.Content != nil && *message.Content != "" {
		assistant.Content.OfString = openaisdk.String(*message.Content)
	}
	assistant.ToolCalls = make([]openaisdk.ChatCompletionMessageToolCallParam, len(message.ToolCalls))
	for i, toolCall := range message.ToolCalls {
		assistant.ToolCalls[i] = openaisdk.ChatCompletionMessageToolCallParam{
			ID: toolCall.ID,
			Function: openaisdk.ChatCompletionMessageToolCallFunctionParam{
				Name:      toolCall.Function.Name,
				Arguments: toolCall.Function.Arguments,
			},
		}
	}

	return openaisdk.ChatCompletionMessageParamUnion{OfAssistant: &assistant}
}

func toSDKTools(tools []gogent.Tool) ([]openaisdk.ChatCompletionToolParam, error) {
	definitions := toOpenAIToolDefinitions(tools)
	sdkTools := make([]openaisdk.ChatCompletionToolParam, len(definitions))

	for i, definition := range definitions {
		var parameters shared.FunctionParameters
		if len(definition.Function.Parameters) > 0 {
			if err := json.Unmarshal(definition.Function.Parameters, &parameters); err != nil {
				return nil, fmt.Errorf("openai: tool %s parameters: %w", definition.Function.Name, err)
			}
		}

		description := param.Opt[string]{}
		if definition.Function.Description != "" {
			description = openaisdk.String(definition.Function.Description)
		}
		sdkTools[i] = openaisdk.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        definition.Function.Name,
				Description: description,
				Parameters:  parameters,
			},
		}
	}

	return sdkTools, nil
}

func toSDKResponseFormat(raw json.RawMessage) (openaisdk.ChatCompletionNewParamsResponseFormatUnion, error) {
	var payload struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{}, fmt.Errorf("openai: response format: %w", err)
	}

	switch payload.Type {
	case "json_object":
		jsonObject := shared.NewResponseFormatJSONObjectParam()
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{OfJSONObject: &jsonObject}, nil
	case "text", "":
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{OfText: &shared.ResponseFormatTextParam{}}, nil
	default:
		return openaisdk.ChatCompletionNewParamsResponseFormatUnion{}, fmt.Errorf("openai: unsupported response format type %q", payload.Type)
	}
}
