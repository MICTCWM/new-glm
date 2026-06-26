package relay

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
)

// ApplyForceSystemPromptToMessages 将强制系统提示词拼接到 OpenAI Chat 请求的 messages 中。
// 如果请求中没有 system message，则插入新的 system message，内容为 constant.ForceSystemPrompt；
// 如果已有 system message，则将 constant.ForceSystemPrompt 拼接到现有内容的最前面（用 "\n" 连接）。
// 处理 string content 和 []MediaContent 数组两种格式。
func ApplyForceSystemPromptToMessages(request *dto.GeneralOpenAIRequest) {
	if request == nil {
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
			Content: constant.ForceSystemPrompt,
		}
		request.Messages = append([]dto.Message{systemMessage}, request.Messages...)
		return
	}
	for i, message := range request.Messages {
		if message.Role != systemRole {
			continue
		}
		if message.IsStringContent() {
			request.Messages[i].SetStringContent(constant.ForceSystemPrompt + "\n" + message.StringContent())
		} else {
			contents := message.ParseContent()
			contents = append([]dto.MediaContent{
				{
					Type: dto.ContentTypeText,
					Text: constant.ForceSystemPrompt,
				},
			}, contents...)
			request.Messages[i].Content = contents
		}
		return
	}
}

// ApplyForceSystemPromptToInstructions 将强制系统提示词拼接到 OpenAIResponsesRequest.Instructions。
// 如果 Instructions 为空，则设置为 ForceSystemPrompt；
// 如果 Instructions 为非空字符串，则拼接为 ForceSystemPrompt + "\n" + 原内容；
// 如果 Instructions 不是简单字符串（如 JSON 对象），则直接替换为 ForceSystemPrompt。
// 返回 Marshal 过程中的错误。
func ApplyForceSystemPromptToInstructions(request *dto.OpenAIResponsesRequest) error {
	if request == nil {
		return nil
	}
	if len(request.Instructions) == 0 {
		b, err := common.Marshal(constant.ForceSystemPrompt)
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
			b, err := common.Marshal(constant.ForceSystemPrompt)
			if err != nil {
				return err
			}
			request.Instructions = b
			return nil
		}
		b, err := common.Marshal(constant.ForceSystemPrompt + "\n" + existing)
		if err != nil {
			return err
		}
		request.Instructions = b
		return nil
	}
	// Instructions 不是简单字符串（可能是对象），直接设置为强制提示词
	b, err := common.Marshal(constant.ForceSystemPrompt)
	if err != nil {
		return err
	}
	request.Instructions = b
	return nil
}
