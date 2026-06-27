package relay

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

// ApplyForceSystemPromptToMessages 将强制系统提示词拼接到 OpenAI Chat 请求的 messages 中。
// 如果请求中没有 system message，则插入新的 system message，内容为对应模型的强制提示词；
// 如果已有 system message，则将强制提示词拼接到现有内容的最前面（用 "\n" 连接）。
// 处理 string content 和 []MediaContent 数组两种格式。
// 若 modelName 对应的模型未配置强制提示词（返回空字符串），则跳过注入。
func ApplyForceSystemPromptToMessages(request *dto.GeneralOpenAIRequest, modelName string) {
	if request == nil {
		return
	}
	prompt := constant.GetForceSystemPrompt(modelName)
	if prompt == "" {
		return
	}
	systemRole := request.GetSystemRoleName()
	containSystem := false
	for _, message := range request.Messages {
		if message.Role == systemRole {
			containSystem = true
			break
		}
	}
	if !containSystem {
		systemMessage := dto.Message{
			Role:    systemRole,
			Content: prompt,
		}
		request.Messages = append([]dto.Message{systemMessage}, request.Messages...)
		return
	}
	for i, message := range request.Messages {
		if message.Role != systemRole {
			continue
		}
		if message.IsStringContent() {
			request.Messages[i].SetStringContent(prompt + "\n" + message.StringContent())
		} else {
			contents := message.ParseContent()
			contents = append([]dto.MediaContent{
				{
					Type: dto.ContentTypeText,
					Text: prompt,
				},
			}, contents...)
			request.Messages[i].Content = contents
		}
		return
	}
}

// ApplyForceSystemPromptToInstructions 将强制系统提示词拼接到 OpenAIResponsesRequest.Instructions。
// 如果 Instructions 为空，则设置为对应模型的强制提示词；
// 如果 Instructions 为非空字符串，则拼接为强制提示词 + "\n" + 原内容；
// 如果 Instructions 不是简单字符串（如 JSON 对象），则直接替换为强制提示词。
// 若 modelName 对应的模型未配置强制提示词（返回空字符串），则跳过注入。
// 返回 Marshal 过程中的错误。
func ApplyForceSystemPromptToInstructions(request *dto.OpenAIResponsesRequest, modelName string) error {
	if request == nil {
		return nil
	}
	prompt := constant.GetForceSystemPrompt(modelName)
	if prompt == "" {
		return nil
	}
	if len(request.Instructions) == 0 {
		b, err := common.Marshal(prompt)
		if err != nil {
			return err
		}
		request.Instructions = b
		return nil
	}
	var existing string
	if err := common.Unmarshal(request.Instructions, &existing); err == nil {
		existing = strings.TrimSpace(existing)
		if existing == "" {
			b, err := common.Marshal(prompt)
			if err != nil {
				return err
			}
			request.Instructions = b
			return nil
		}
		b, err := common.Marshal(prompt + "\n" + existing)
		if err != nil {
			return err
		}
		request.Instructions = b
		return nil
	}
	// Instructions 不是简单字符串（可能是对象），直接设置为强制提示词
	b, err := common.Marshal(prompt)
	if err != nil {
		return err
	}
	request.Instructions = b
	return nil
}
