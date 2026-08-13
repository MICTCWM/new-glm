package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelNoImageModelsRoundTrip(t *testing.T) {
	original := ModelNoImageModels
	defer func() { ModelNoImageModels = original }()

	ModelNoImageModelsFromString("gpt-4o-mini\n  \nkimi-latest\n\nglm-5.2  ")
	require.Equal(t, []string{"gpt-4o-mini", "kimi-latest", "glm-5.2"}, ModelNoImageModels)

	// ToString 用换行拼接，可再次解析回同样结果
	ModelNoImageModelsFromString(ModelNoImageModelsToString())
	require.Equal(t, []string{"gpt-4o-mini", "kimi-latest", "glm-5.2"}, ModelNoImageModels)
}

func TestIsModelNoImage(t *testing.T) {
	original := ModelNoImageModels
	defer func() { ModelNoImageModels = original }()

	ModelNoImageModelsFromString("gpt-4o-mini\nkimi-latest")

	require.True(t, IsModelNoImage("gpt-4o-mini"))
	require.True(t, IsModelNoImage("kimi-latest"))
	require.False(t, IsModelNoImage("gpt-4o"))
	require.False(t, IsModelNoImage(""))
	require.False(t, IsModelNoImage("GPT-4o-mini")) // 大小写敏感，不做模糊匹配
}

func TestModelNoImageModelsEmpty(t *testing.T) {
	original := ModelNoImageModels
	defer func() { ModelNoImageModels = original }()

	// 默认空列表：所有模型都支持图片
	require.Empty(t, ModelNoImageModels)
	require.False(t, IsModelNoImage("any-model"))

	// 空字符串解析后仍为空
	ModelNoImageModelsFromString("")
	require.Empty(t, ModelNoImageModels)
}
